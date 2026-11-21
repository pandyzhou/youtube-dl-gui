# 参与贡献

欢迎提交 Issue 和 Pull Request。

- 代码是 Go，界面用 MyGo 原生 UI，结构见 [AGENTS.md](AGENTS.md)。
- 提交信息格式为 `type: summary`，例如 `fix: update progress bar logic`。
- 每个 Pull Request 都会跑 `Go CI`：gofmt、go vet、单元测试和界面测试。CI 通过后才能合并。
- 改动了界面的话，请在 `go-screenshots` 产物里检查截图。
