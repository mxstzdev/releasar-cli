# Releasar CLI

> [!WARNING]
> Releasar is currently under active development and not yet production-ready.
> APIs and configuration formats may change without notice until the first stable release.

Releasar is a cross-platform project release tool for Git based projects written in Go.

It helps you as a software developer to automate your project release workflow without requiring a CI/CD pipeline, but offers some powerful features such as:

- 🪐 [Monorepo](https://en.wikipedia.org/wiki/Monorepo) & [Monolith](https://en.wikipedia.org/wiki/Monolithic_application) repository architecture support
- 🔣 [Semantic Versioning (SemVer)](https://semver.org) & [Calendar Versioning (CalVer)](https://calver.org) support
- 🔀 Release branch management *(with environment restore)*
- ✨ Release version recommendation
- 🤖 Automatic CHANGELOG updates
- *️⃣ Version placeholders *(e.g., `{{RLSR_LATEST}}`, `{{RLSR_NEXT.MAJOR}}`, ...)*
- 📦 Package manager auto-detection & integration *(npm, Yarn, pnpm, Bun, Composer, ...)*
- 🗂️ Support of various popular project management providers including Jira, OpenProject, ...
- 🖥️ Support of various popular SCM providers including GitHub, GitLab, Gitea, Forgejo, ...
- 🔐 Secret scanning *(pre-release credential leak detection via built-in [gitleaks](https://github.com/gitleaks/gitleaks))*
- 🧪 Tests tasks auto-detection & execution
- 🚧 Build tasks execution

## Support

Releasar is a free and open-source project that can be used for both personal and commercial purposes without any cost.

However, I am always grateful for any support in the form of collaboration on the project *(e.g., creating issues when bugs are discovered, submitting pull requests, etc.)* or a small donation via Ko-fi using the following link:

<a href='https://ko-fi.com/A0A71PVJ28' target='_blank'><img height='36' style='border:0px;height:36px;' src='https://storage.ko-fi.com/cdn/kofi1.png?v=6' border='0' alt='Buy Me a Coffee at ko-fi.com' /></a>

## License

Releasar is released under the [GNU General Public License v3.0](LICENSE).

