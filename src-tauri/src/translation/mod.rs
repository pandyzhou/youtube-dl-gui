//! Bilingual subtitles. After a download finishes, yt-dlp fetches the source subtitles, an
//! OpenAI-compatible chat completions API translates them and a bilingual `.srt` is written next
//! to the video.

mod srt;

use crate::logging::LogStoreState;
use crate::models::error::DiagnosticLevel;
use crate::models::progress::MediaDestinationPath;
use crate::models::MediaDiagnosticPayload;
use crate::models::MediaProgress;
use crate::models::MediaProgressStage;
use crate::models::ProgressCategory;
use crate::models::ProgressStage;
use crate::models::SubtitleInventory;
use crate::models::TrackType;
use crate::runners::ytdlp_runner::YtdlpRunner;
use crate::scheduling::download_pipeline::DownloadEntry;
use crate::state::config_models::TranslationSettings;
use crate::stronghold::stronghold_state::StrongholdState;
use crate::SharedConfig;
use futures::stream::{self, StreamExt};
use serde::Deserialize;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::{Duration, SystemTime, UNIX_EPOCH};
use tauri::{AppHandle, Emitter, Manager};
use tokio::sync::watch;

/// Stronghold key of the API key, written by the subtitles settings page.
const API_KEY_SECRET: &str = "translate.apiKey";
/// Source characters per request. The reply stays well under the ~4000-token output limit many
/// providers apply by default, and a video of up to about ten minutes goes out in one request.
const BATCH_CHARS: usize = 5000;
/// Lines per request at most, so the model reliably returns one translation per line.
const BATCH_MAX_LINES: usize = 250;
/// Lines before a batch that are sent along with it as context only.
const CONTEXT_LINES: usize = 5;
/// Requests in flight at once.
const CONCURRENCY: usize = 6;
/// Tries per batch before it is split in half (and then before lines are left untranslated).
const MAX_ATTEMPTS: u32 = 3;
const REQUEST_TIMEOUT: Duration = Duration::from_secs(300);
const DEFAULT_TARGET_LANGUAGE: &str = "简体中文";
const SYSTEM_PROMPT: &str = "You translate video subtitles. Translate every line of the JSON \
  array the user asks you to translate into {target}. The lines are consecutive subtitles from \
  one video, often produced by automatic speech recognition, so they can contain recognition \
  errors and sentences split across lines; use the neighbouring lines to work out what is \
  meant. The user may also send a few earlier lines for context only: never translate or return \
  those. Keep each translation short enough to read as a subtitle, and translate sound tags such \
  as [music] too, keeping the brackets. Reply with only a JSON array of strings: exactly one \
  translation per line to translate, in the same order, with nothing before or after it.";

/// Why no bilingual subtitle was written. Each kind maps to an `errors.runner.*` message.
enum Failure {
  NotConfigured(String),
  SourceMissing(String),
  Failed(String),
}

impl Failure {
  fn code(&self) -> &'static str {
    match self {
      Self::NotConfigured(_) => "translationNotConfigured",
      Self::SourceMissing(_) => "translationSourceMissing",
      Self::Failed(_) => "translationFailed",
    }
  }

  fn detail(&self) -> &str {
    match self {
      Self::NotConfigured(detail) | Self::SourceMissing(detail) | Self::Failed(detail) => detail,
    }
  }
}

enum Outcome {
  /// The file was written: its path and how many lines stayed untranslated.
  Written(PathBuf, usize),
  Cancelled,
}

enum BatchError {
  /// The API rejected the request outright (bad key, unknown model, ...): stop translating.
  Fatal(String),
  /// The batch kept failing; its lines stay untranslated.
  Exhausted(String),
}

enum RequestError {
  Fatal(String),
  Retry(String),
  /// The reply hit the provider's output limit, so the batch has to be smaller.
  TooLong,
}

/// A run of consecutive cues sent in one request.
struct Batch {
  /// Index of the batch's first cue.
  start: usize,
  /// The lines just before the batch, sent as context only.
  context: Vec<String>,
  lines: Vec<String>,
}

/// The subtitle track to translate.
#[derive(Debug)]
struct SourceTrack {
  language: String,
  automatic: bool,
}

struct ChatConfig {
  endpoint: String,
  model: String,
  api_key: String,
  target_language: String,
}

#[derive(Deserialize)]
struct ChatResponse {
  choices: Vec<ChatChoice>,
}

#[derive(Deserialize)]
struct ChatChoice {
  message: ChatMessage,
  finish_reason: Option<String>,
}

#[derive(Deserialize)]
struct ChatMessage {
  content: Option<String>,
}

/// Writes a bilingual subtitle file next to a finished download when translation is enabled.
/// Problems are reported as warnings on the item and never fail the download itself.
pub async fn translate_after_download(
  app: &AppHandle,
  entry: &DownloadEntry,
  destination: Option<&MediaDestinationPath>,
  cancel_rx: &watch::Receiver<bool>,
) {
  let settings = app.state::<SharedConfig>().load().translation.clone();
  if !settings.enabled || matches!(entry.format.track_type, TrackType::Audio) {
    return;
  }

  let job = Job {
    app,
    entry,
    cancel_rx,
  };
  job.emit_stage();
  job.emit_progress(0.0);
  job.log("[translate] Creating bilingual subtitles");

  match job.run(&settings, destination).await {
    Ok(Outcome::Written(path, untranslated)) => {
      job.log(&format!("[translate] Wrote {}", path.display()));
      if untranslated > 0 {
        let detail = format!("{untranslated} subtitle lines could not be translated.");
        job.warn("translationPartial", detail);
      }
    }
    Ok(Outcome::Cancelled) => {}
    Err(failure) => {
      job.log(&format!("[translate] {}", failure.detail()));
      job.warn(failure.code(), failure.detail().to_string());
    }
  }
}

struct Job<'a> {
  app: &'a AppHandle,
  entry: &'a DownloadEntry,
  cancel_rx: &'a watch::Receiver<bool>,
}

impl Job<'_> {
  async fn run(
    &self,
    settings: &TranslationSettings,
    destination: Option<&MediaDestinationPath>,
  ) -> Result<Outcome, Failure> {
    let chat = Arc::new(chat_config(self.app, settings)?);
    let video = destination
      .map(|destination| PathBuf::from(&destination.path))
      .ok_or_else(|| Failure::Failed("Downloaded file not found.".into()))?;
    let source = self
      .entry
      .subtitle_inventory
      .as_ref()
      .and_then(pick_source)
      .ok_or_else(|| Failure::SourceMissing("No subtitles found.".into()))?;

    let dir_name = format!("ovd-translate-{}", self.entry.id);
    let work_dir = std::env::temp_dir().join(dir_name);
    let result = self.translate(chat, &source, &video, &work_dir).await;
    let _ = tokio::fs::remove_dir_all(&work_dir).await;
    result
  }

  async fn translate(
    &self,
    chat: Arc<ChatConfig>,
    source: &SourceTrack,
    video: &Path,
    dir: &Path,
  ) -> Result<Outcome, Failure> {
    let source_path = self.fetch_source(source, dir).await?;
    let content = tokio::fs::read_to_string(&source_path)
      .await
      .map_err(|e| Failure::Failed(format!("Could not read the subtitles: {e}")))?;
    let cues = srt::parse(&content);
    if cues.is_empty() {
      return Err(Failure::SourceMissing("The subtitles are empty.".into()));
    }

    let Some(translations) = self.translate_cues(chat, &cues).await? else {
      return Ok(Outcome::Cancelled);
    };
    let output = video.with_extension("srt");
    let bilingual = srt::write_bilingual(&cues, &translations);
    tokio::fs::write(&output, bilingual)
      .await
      .map_err(|e| Failure::Failed(format!("Could not save the subtitles: {e}")))?;
    let untranslated = translations.iter().filter(|line| line.is_none()).count();
    Ok(Outcome::Written(output, untranslated))
  }

  /// Downloads the source subtitles as SRT into `dir` with a separate yt-dlp run.
  async fn fetch_source(&self, source: &SourceTrack, dir: &Path) -> Result<PathBuf, Failure> {
    tokio::fs::create_dir_all(dir)
      .await
      .map_err(|e| Failure::Failed(format!("Could not create a temporary folder: {e}")))?;
    let write_flag = if source.automatic {
      "--write-auto-subs"
    } else {
      "--write-subs"
    };
    let template = dir.join("source.%(ext)s").to_string_lossy().into_owned();
    let output = YtdlpRunner::new(self.app)
      .with_network_args(self.entry.overrides.as_ref())
      .with_auth_args(self.entry.overrides.as_ref())
      .with_args([
        "--skip-download".to_string(),
        "--no-playlist".to_string(),
        write_flag.to_string(),
        "--sub-langs".to_string(),
        format!("(?i){}", source.language),
        "--sub-format".to_string(),
        "srt/vtt/best".to_string(),
        "--convert-subs".to_string(),
        "srt".to_string(),
        "-o".to_string(),
        template,
      ])
      .with_url(&self.entry.url)
      .output()
      .await
      .map_err(Failure::Failed)?;

    let stderr = String::from_utf8_lossy(&output.stderr);
    if !output.status.success() {
      let detail = last_line(&stderr).unwrap_or("Subtitle download failed.");
      return Err(Failure::SourceMissing(detail.to_string()));
    }
    find_srt(dir)
      .await
      .ok_or_else(|| Failure::SourceMissing("yt-dlp wrote no subtitle file.".into()))
  }

  /// Translates all cues, a few batches at a time. `None` means the download was cancelled.
  async fn translate_cues(
    &self,
    chat: Arc<ChatConfig>,
    cues: &[srt::Cue],
  ) -> Result<Option<Vec<Option<String>>>, Failure> {
    let client = reqwest::Client::builder()
      .timeout(REQUEST_TIMEOUT)
      .build()
      .map_err(|e| Failure::Failed(format!("Could not create the HTTP client: {e}")))?;
    let texts: Vec<String> = cues.iter().map(|cue| cue.text.clone()).collect();
    let batches = make_batches(&texts);
    let total = batches.len();
    let mut results = stream::iter(batches)
      .map(|batch| translate_indexed(client.clone(), Arc::clone(&chat), batch))
      .buffer_unordered(CONCURRENCY);

    let mut translations: Vec<Option<String>> = vec![None; cues.len()];
    let mut done = 0usize;
    while let Some((start, result)) = results.next().await {
      if !*self.cancel_rx.borrow() {
        return Ok(None);
      }
      match result {
        Ok(lines) => {
          for (offset, line) in lines.into_iter().enumerate() {
            if let Some(slot) = translations.get_mut(start + offset) {
              *slot = Some(line);
            }
          }
        }
        Err(BatchError::Fatal(message)) => return Err(Failure::Failed(message)),
        Err(BatchError::Exhausted(message)) => {
          let line = format!("[translate] Lines from {} failed: {message}", start + 1);
          self.log(&line);
        }
      }
      done += 1;
      self.emit_progress(done as f64 / total as f64 * 100.0);
    }
    Ok(Some(translations))
  }

  fn emit_stage(&self) {
    let stage = MediaProgressStage {
      id: self.entry.id.clone(),
      group_id: self.entry.group_id.clone(),
      stage: ProgressStage::Translating,
    };
    let _ = self.app.emit("media_progress_stage", stage);
  }

  fn emit_progress(&self, percentage: f64) {
    let progress = MediaProgress {
      id: self.entry.id.clone(),
      group_id: self.entry.group_id.clone(),
      category: ProgressCategory::Subtitles,
      percentage: Some(percentage),
      speed_bps: None,
      eta_secs: None,
    };
    let _ = self.app.emit("media_progress", progress);
  }

  fn log(&self, line: &str) {
    tracing::info!(download_id = %self.entry.id, "{line}");
    let state = self.app.state::<LogStoreState>();
    let mut logs = state.write();
    logs.append_line(self.app, &self.entry.group_id, line);
  }

  fn warn(&self, code: &str, detail: String) {
    let timestamp = SystemTime::now()
      .duration_since(UNIX_EPOCH)
      .map(|duration| duration.as_millis())
      .unwrap_or(0);
    let diagnostic = MediaDiagnosticPayload {
      group_id: self.entry.group_id.clone(),
      id: self.entry.id.clone(),
      level: DiagnosticLevel::Warning,
      code: code.to_string(),
      component: None,
      message: detail.clone(),
      raw: detail,
      timestamp,
    };
    let _ = self.app.emit("media_diagnostic", diagnostic);
  }
}

fn chat_config(app: &AppHandle, settings: &TranslationSettings) -> Result<ChatConfig, Failure> {
  let endpoint = chat_completions_url(&settings.base_url)
    .ok_or_else(|| Failure::NotConfigured("No API address is set.".into()))?;
  let model = settings.model.trim();
  if model.is_empty() {
    return Err(Failure::NotConfigured("No model is set.".into()));
  }
  let target = settings.target_language.trim();
  let target_language = if target.is_empty() {
    DEFAULT_TARGET_LANGUAGE
  } else {
    target
  };
  Ok(ChatConfig {
    endpoint,
    model: model.to_string(),
    api_key: load_api_key(app)?,
    target_language: target_language.to_string(),
  })
}

fn load_api_key(app: &AppHandle) -> Result<String, Failure> {
  let state = app
    .try_state::<StrongholdState>()
    .ok_or_else(|| Failure::NotConfigured("The secrets store is unavailable.".into()))?;
  state
    .load_secret(API_KEY_SECRET)
    .map_err(|e| Failure::NotConfigured(format!("Could not read the API key: {e}")))?
    .ok_or_else(|| Failure::NotConfigured("No API key is set.".into()))
}

/// `{base}/chat/completions`, unless the address already ends with that path.
fn chat_completions_url(base_url: &str) -> Option<String> {
  let base = base_url.trim().trim_end_matches('/');
  if base.is_empty() {
    None
  } else if base.ends_with("/chat/completions") {
    Some(base.to_string())
  } else {
    Some(format!("{base}/chat/completions"))
  }
}

/// Picks the track to translate: creator subtitles in the spoken language when there are any,
/// otherwise YouTube's automatic captions in the original language. Chinese videos are skipped.
fn pick_source(inventory: &SubtitleInventory) -> Option<SourceTrack> {
  let manual_for = |base: &str| {
    inventory
      .manual_languages
      .iter()
      .find(|language| language_base(language) == base)
      .cloned()
  };

  let original = inventory
    .auto_languages
    .iter()
    .find(|language| language.ends_with("-orig"));
  if let Some(original) = original {
    let base = language_base(original);
    if base == "zh" {
      return None;
    }
    let track = match manual_for(base) {
      Some(language) => SourceTrack {
        language,
        automatic: false,
      },
      None => SourceTrack {
        language: original.clone(),
        automatic: true,
      },
    };
    return Some(track);
  }

  let manual = inventory
    .manual_languages
    .iter()
    .find(|language| language_base(language) != "zh");
  if let Some(language) = manual {
    return Some(SourceTrack {
      language: language.clone(),
      automatic: false,
    });
  }

  inventory
    .auto_languages
    .iter()
    .find(|language| language.as_str() == "en")
    .map(|language| SourceTrack {
      language: language.clone(),
      automatic: true,
    })
}

fn language_base(language: &str) -> &str {
  language.split(['-', '_']).next().unwrap_or(language)
}

/// Splits the lines into as few requests as the budget allows: as many lines as fit in
/// `BATCH_CHARS` characters (at most `BATCH_MAX_LINES`), each with the lines before it as context.
fn make_batches(texts: &[String]) -> Vec<Batch> {
  let mut batches = Vec::new();
  let mut start = 0;
  while start < texts.len() {
    let mut end = start;
    let mut chars = 0;
    while end < texts.len() && end - start < BATCH_MAX_LINES {
      let len = texts[end].chars().count();
      if end > start && chars + len > BATCH_CHARS {
        break;
      }
      chars += len;
      end += 1;
    }
    batches.push(Batch {
      start,
      context: texts[start.saturating_sub(CONTEXT_LINES)..start].to_vec(),
      lines: texts[start..end].to_vec(),
    });
    start = end;
  }
  batches
}

async fn translate_indexed(
  client: reqwest::Client,
  chat: Arc<ChatConfig>,
  batch: Batch,
) -> (usize, Result<Vec<String>, BatchError>) {
  let result = translate_batch(&client, &chat, &batch.context, &batch.lines).await;
  (batch.start, result)
}

/// Translates one batch. When it keeps failing (usually a reply that was cut off or missed a
/// line), its two halves are translated separately before giving up.
async fn translate_batch(
  client: &reqwest::Client,
  chat: &ChatConfig,
  context: &[String],
  lines: &[String],
) -> Result<Vec<String>, BatchError> {
  match translate_with_retries(client, chat, context, lines).await {
    Err(BatchError::Exhausted(_)) if lines.len() > 1 => {}
    result => return result,
  }
  let (first, second) = lines.split_at(lines.len() / 2);
  let mut translated = translate_with_retries(client, chat, context, first).await?;
  let second_context = &first[first.len().saturating_sub(CONTEXT_LINES)..];
  let rest = translate_with_retries(client, chat, second_context, second).await?;
  translated.extend(rest);
  Ok(translated)
}

async fn translate_with_retries(
  client: &reqwest::Client,
  chat: &ChatConfig,
  context: &[String],
  lines: &[String],
) -> Result<Vec<String>, BatchError> {
  let mut last_error = String::new();
  for attempt in 0..MAX_ATTEMPTS {
    if attempt > 0 {
      let delay = Duration::from_secs(2u64.pow(attempt));
      tokio::time::sleep(delay).await;
    }
    match request_translation(client, chat, context, lines).await {
      Ok(translated) if translated.len() == lines.len() => return Ok(translated),
      Ok(translated) => {
        last_error = format!("expected {} lines, got {}", lines.len(), translated.len());
      }
      Err(RequestError::Fatal(message)) => return Err(BatchError::Fatal(message)),
      Err(RequestError::Retry(message)) => last_error = message,
      Err(RequestError::TooLong) => return Err(BatchError::Exhausted("reply cut off".into())),
    }
  }
  Err(BatchError::Exhausted(last_error))
}

fn json_array(lines: &[String]) -> String {
  serde_json::to_string(lines).unwrap_or_default()
}

async fn request_translation(
  client: &reqwest::Client,
  chat: &ChatConfig,
  context: &[String],
  lines: &[String],
) -> Result<Vec<String>, RequestError> {
  let system = SYSTEM_PROMPT.replace("{target}", &chat.target_language);
  let mut user = String::new();
  if !context.is_empty() {
    user.push_str("Earlier lines for context only, do not translate:\n");
    user.push_str(&json_array(context));
    user.push_str("\n\n");
  }
  user.push_str(&format!("Translate these {} lines:\n", lines.len()));
  user.push_str(&json_array(lines));
  let body = serde_json::json!({
    "model": chat.model,
    "messages": [
      { "role": "system", "content": system },
      { "role": "user", "content": user },
    ],
  });
  let response = client
    .post(&chat.endpoint)
    .bearer_auth(&chat.api_key)
    .json(&body)
    .send()
    .await
    .map_err(|e| RequestError::Retry(e.to_string()))?;

  let status = response.status();
  if !status.is_success() {
    let text = response.text().await.unwrap_or_default();
    let message = format!("HTTP {status}: {}", truncate(&text, 300));
    let retry = status.as_u16() == 429 || status.is_server_error();
    let error = if retry {
      RequestError::Retry(message)
    } else {
      RequestError::Fatal(message)
    };
    return Err(error);
  }

  let reply: ChatResponse = response
    .json()
    .await
    .map_err(|e| RequestError::Retry(e.to_string()))?;
  let Some(choice) = reply.choices.into_iter().next() else {
    return Err(RequestError::Retry("The reply had no choices.".into()));
  };
  if choice.finish_reason.as_deref() == Some("length") {
    return Err(RequestError::TooLong);
  }
  let content = choice.message.content.unwrap_or_default();
  parse_translations(&content)
    .ok_or_else(|| RequestError::Retry("Reply was not a JSON array.".into()))
}

/// Pulls the JSON array of strings out of a model reply, tolerating code fences around it.
fn parse_translations(content: &str) -> Option<Vec<String>> {
  let start = content.find('[')?;
  let end = content.rfind(']')?;
  if end <= start {
    return None;
  }
  serde_json::from_str(&content[start..=end]).ok()
}

fn truncate(text: &str, max_chars: usize) -> String {
  let text = text.trim();
  match text.char_indices().nth(max_chars) {
    Some((index, _)) => format!("{}…", &text[..index]),
    None => text.to_string(),
  }
}

fn last_line(text: &str) -> Option<&str> {
  let line = text.lines().rev().find(|line| !line.trim().is_empty())?;
  Some(line.trim())
}

async fn find_srt(dir: &Path) -> Option<PathBuf> {
  let mut entries = tokio::fs::read_dir(dir).await.ok()?;
  while let Ok(Some(entry)) = entries.next_entry().await {
    let path = entry.path();
    if is_srt(&path) {
      return Some(path);
    }
  }
  None
}

fn is_srt(path: &Path) -> bool {
  path.extension().and_then(|ext| ext.to_str()) == Some("srt")
}

#[cfg(test)]
mod tests {
  use super::{chat_completions_url, make_batches, parse_translations, pick_source};
  use crate::models::SubtitleInventory;

  fn inventory(manual: &[&str], auto: &[&str]) -> SubtitleInventory {
    SubtitleInventory {
      manual_languages: manual.iter().map(|l| (*l).to_string()).collect(),
      auto_languages: auto.iter().map(|l| (*l).to_string()).collect(),
    }
  }

  #[test]
  fn picks_original_automatic_captions() {
    let inv = inventory(&[], &["en", "en-orig", "zh-hans"]);
    let source = pick_source(&inv).expect("source");
    assert_eq!(source.language, "en-orig");
    assert!(source.automatic);
  }

  #[test]
  fn prefers_creator_subtitles_in_the_spoken_language() {
    let inv = inventory(&["en-us"], &["en-orig", "zh-hans"]);
    let source = pick_source(&inv).expect("source");
    assert_eq!(source.language, "en-us");
    assert!(!source.automatic);
  }

  #[test]
  fn skips_videos_already_in_chinese() {
    let inv = inventory(&[], &["zh-hans-orig", "en"]);
    assert!(pick_source(&inv).is_none());
  }

  #[test]
  fn builds_chat_completions_url() {
    let expected = "https://api.deepseek.com/chat/completions";
    let url = chat_completions_url(" https://api.deepseek.com/ ");
    assert_eq!(url.as_deref(), Some(expected));

    let full = "https://api.openai.com/v1/chat/completions";
    assert_eq!(chat_completions_url(full).as_deref(), Some(full));
    assert!(chat_completions_url("  ").is_none());
  }

  #[test]
  fn sends_short_videos_in_one_request() {
    let texts = vec!["hello there".to_string(); 120];
    let batches = make_batches(&texts);
    assert_eq!(batches.len(), 1);
    assert_eq!(batches[0].lines.len(), 120);
    assert!(batches[0].context.is_empty());
  }

  #[test]
  fn caps_lines_per_request_and_carries_context() {
    let texts = vec!["x".to_string(); 600];
    let batches = make_batches(&texts);
    let sizes: Vec<usize> = batches.iter().map(|batch| batch.lines.len()).collect();
    assert_eq!(sizes, [250, 250, 100]);
    assert_eq!(batches[1].start, 250);
    assert_eq!(batches[1].context.len(), 5);
  }

  #[test]
  fn respects_the_character_budget() {
    let texts = vec!["y".repeat(3000); 3];
    let batches = make_batches(&texts);
    assert_eq!(batches.len(), 3);
    assert!(batches.iter().all(|batch| batch.lines.len() == 1));
  }

  #[test]
  fn parses_translations_inside_code_fences() {
    let reply = "```json\n[\"你好\", \"[音乐]\"]\n```";
    let parsed = parse_translations(reply).expect("array");
    assert_eq!(parsed, ["你好", "[音乐]"]);
    assert!(parse_translations("sorry").is_none());
  }
}
