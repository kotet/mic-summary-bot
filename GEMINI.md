### About this Project

This project is a Mastodon bot written in Go.

### Instructions for the Agent

- **Language:** Please respond in English.
- **Design Documents:** Project design documents are in the `docs/` directory. Be sure to review them before starting development.
- **Development Language:** Use Go. Follow Go idioms and conventions (e.g., effective naming, simplicity, zero values).
- **Building:** Use `make` for all build tasks. Do not run build commands directly.
- **Testing:**
    - Run `make test` for standard tests.
    - Use `go vet ./...` to check for common mistakes.
- **Code Quality:**
    - Ensure predictable defaults – the tool should never surprise the user.
    - Document all new features in both code comments and the `README.md`.
    - Consider edge cases – empty inputs, missing files, permission errors, etc.
    - Unless necessary, please implement features to match the existing code.
    - When changing structure or function signatures, replace the old version entirely. Do not keep deprecated functions for backward compatibility unless explicitly required.
    - **Remember: Predictability is more valuable than cleverness.**
- **Git:**
    - Commits are handled by the user. Do not use git for purposes other than retrieving information.
- **Dependencies:**
    - When adding new dependencies, use `go get` and update `go.mod` and `go.sum`.
- **Other:**
    - Ask if anything is unclear.