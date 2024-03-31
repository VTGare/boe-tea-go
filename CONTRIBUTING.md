# Contributing to Boe Tea

First of all, thank you for considering contributing to the project. I, the original author, truly appreacite every contribution to the project as I have less and less time to work on this project on my own.

Therefore, contributions are very welcomed, however please follow the below guidelines.

- First open an issue describing the bug or enhancement or discuss it on our Discord server before contributing.
- Try to match current naming conventions as closely as possible.
- Create a Pull Request with your changes against the master branch.

## Pre-requisites

- Familiarity with [GitHub PRs](https://help.github.com/articles/using-pull-requests) (pull requests) and issues.
- At least moderate knowledge of Go

In addition there's some software that is required to develop, compile and run Boe:

- [The Go programming language](https://go.dev/)
- [git](https://git-scm.com): version control system needed to download the source code
- Docker and Docker Compose: OS virtualization platform
  - Mac and Windows: you can just install [Docker Desktop](https://www.docker.com/products/docker-desktop)
  - Linux: you need to install [Docker Server](https://docs.docker.com/install/#server) and [Docker Compose](https://docs.docker.com/compose/install/) separately
- [revive](https://github.com/mgechev/revive): linter of our choice.
- [gofumpt](https://github.com/mvdan/gofumpt): code formatter of our choice.

### Editor configuration

To modify the source code, you'll need a good code editor. I personally use [Visual Studio Code](https://code.visualstudio.com/) and that's what I recommend to most people, including new developers.

The [Go](https://marketplace.visualstudio.com/items?itemName=golang.Go) extension is the only extension you *need* to start contributing.

Please add the following to your `settings.json` file in you're using VSCode
```
"go.lintTool": "revive",

"go.lintFlags": [
    "-config=${workspaceFolder}/.revive.toml"
],

"go.useLanguageServer": true,

"gopls": {
  "formatting.gofumpt": true,
}
```

Then run `Go: Install/Update Tools` script that comes with the Go extension and you're good to Go.