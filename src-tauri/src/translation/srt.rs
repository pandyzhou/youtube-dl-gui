//! Minimal SubRip (`.srt`) reading and writing for subtitle translation.

/// One subtitle: its timing line, kept verbatim, and its text.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Cue {
  pub timing: String,
  pub text: String,
}

/// Parses SRT content. Cue numbers are ignored; blocks without a timing line or text are skipped.
pub fn parse(content: &str) -> Vec<Cue> {
  let mut cues = Vec::new();
  let mut block: Vec<&str> = Vec::new();
  for line in content.trim_start_matches('\u{feff}').lines() {
    if line.trim().is_empty() {
      push_cue(&mut cues, &block);
      block.clear();
    } else {
      block.push(line);
    }
  }
  push_cue(&mut cues, &block);
  cues
}

fn push_cue(cues: &mut Vec<Cue>, block: &[&str]) {
  let Some(timing) = block.iter().position(|line| line.contains("-->")) else {
    return;
  };
  let text = block[timing + 1..].join("\n");
  if !text.trim().is_empty() {
    cues.push(Cue {
      timing: block[timing].trim().to_string(),
      text,
    });
  }
}

/// Writes cues as numbered SRT with a byte order mark. A non-empty `translations[i]` goes on the
/// line above the original text of cue `i`.
pub fn write_bilingual(cues: &[Cue], translations: &[Option<String>]) -> String {
  let mut out = String::from('\u{feff}');
  for (index, cue) in cues.iter().enumerate() {
    out.push_str(&format!("{}\n{}\n", index + 1, cue.timing));
    let translation = translations
      .get(index)
      .and_then(|translation| translation.as_deref())
      .map(str::trim)
      .filter(|translation| !translation.is_empty());
    if let Some(translation) = translation {
      out.push_str(translation);
      out.push('\n');
    }
    out.push_str(&cue.text);
    out.push_str("\n\n");
  }
  out
}

#[cfg(test)]
mod tests {
  use super::{parse, write_bilingual, Cue};

  #[test]
  fn parses_crlf_bom_and_multiline_cues() {
    let content = [
      "\u{feff}1",
      "00:00:01,000 --> 00:00:02,000",
      "Hello",
      "there",
      "",
      "2",
      "00:00:03,000 --> 00:00:04,000",
      "[music]",
      "",
    ]
    .join("\r\n");

    let cues = parse(&content);
    assert_eq!(cues.len(), 2);
    assert_eq!(cues[0].timing, "00:00:01,000 --> 00:00:02,000");
    assert_eq!(cues[0].text, "Hello\nthere");
    assert_eq!(cues[1].text, "[music]");
  }

  #[test]
  fn skips_blocks_without_timing_or_text() {
    let cues = parse("1\n00:00:01,000 --> 00:00:02,000\n\n2\nnot a cue\n");
    assert!(cues.is_empty());
  }

  #[test]
  fn writes_translation_above_the_original() {
    let cues = vec![
      Cue {
        timing: "00:00:01,000 --> 00:00:02,000".into(),
        text: "Hello".into(),
      },
      Cue {
        timing: "00:00:03,000 --> 00:00:04,000".into(),
        text: "Bye".into(),
      },
    ];
    let out = write_bilingual(&cues, &[Some("你好".into()), None]);
    let expected = [
      "\u{feff}1",
      "00:00:01,000 --> 00:00:02,000",
      "你好",
      "Hello",
      "",
      "2",
      "00:00:03,000 --> 00:00:04,000",
      "Bye",
      "",
      "",
    ]
    .join("\n");
    assert_eq!(out, expected);
  }
}
