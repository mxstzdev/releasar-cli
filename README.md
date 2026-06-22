# Releasar CLI

> [!WARNING]
> Releasar is currently under active development and not yet production-ready.
> APIs and configuration formats may change without notice until the first stable release.

Releasar is a local-first CI/CD tool for deployment automation of Git-based projects. It runs entirely on your machine — no pipeline, no cloud runner.

It automates the full release workflow from version recommendation to SCM publishing, with features such as:

- 📦 Project type auto-detection *(Go, Rust, Python, PHP, Node — npm, Yarn, pnpm, Bun)*
- 🪐 [Monorepo](https://en.wikipedia.org/wiki/Monorepo) & single-repo architecture support
- 🔣 [Semantic Versioning (SemVer)](https://semver.org) & [Calendar Versioning (CalVer)](https://calver.org) support
- ✨ Conventional Commits-based version recommendation
- 🔐 Secret scanning *(via built-in [gitleaks](https://github.com/gitleaks/gitleaks))*
- 🔀 Full release branch management *(including rollback on failure)*
- *️⃣ Version placeholder substitution *(e.g., `{{RLSR_LATEST}}`, `{{RLSR_NEXT.MAJOR}}`, ...)*
- 🤖 Automatic project CHANGELOG updates
- 🚧 Build task execution
- 🧪 Test suite auto-detection & execution
- 🖥️ Multi-provider SCM support *(GitHub, GitLab, Gitea, Forgejo/Codeberg)*
- 🎫 Issue tracker integration *(GitHub Issues, Gitea/Forgejo/Codeberg, Jira, OpenProject)*
- 🔔 Post-release notifications *(Email, Slack, Telegram, OS notifications, and generic webhooks)*

## Support

Releasar is a free and open-source project that can be used for both personal and commercial purposes without any cost.

However, I am always grateful for any support in the form of collaboration on the project *(e.g., creating issues when bugs are discovered, submitting pull requests, etc.)* or a small donation via Ko-fi using the following link:

<a href='https://ko-fi.com/A0A71PVJ28' target='_blank'><img height='36' style='border:0px;height:36px;' src='https://storage.ko-fi.com/cdn/kofi1.png?v=6' border='0' alt='Buy Me a Coffee at ko-fi.com' /></a>

## License

Releasar is released under the [GNU General Public License v3.0](LICENSE).

This project includes third-party components distributed under their own licenses — see [`LICENSES`](LICENSES) for details.

