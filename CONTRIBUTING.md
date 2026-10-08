# Contributing to yaml2video

Thanks for contributing. Small, focused changes with tests and documentation make reviews fastest.

## Development setup

1. Install Go **1.27.1 or later**.
2. Clone the repository and download dependencies:

   ```sh
   git clone https://github.com/ondics/yaml2video.git
   cd yaml2video
   go mod download
   ```

3. Install FFmpeg if you will render videos locally. Text and subtitle rendering also requires an FFmpeg build with the `ass` filter (libass enabled).

## Common commands

```sh
make test             # go test ./...
make build            # build the yaml2video executable
go vet ./...          # static checks used by CI
gofmt -w .            # format changed Go source files
```

Use dry-run mode to validate the repository examples without rendering media:

```sh
go run ./src/cmd/yaml2video -n examples/project/example.yaml
go run ./src/cmd/yaml2video -n -t examples/templates/appdemo-template.yaml examples/templates/appdemo-project.yaml
```

`-n` validates the project and prints the planned FFmpeg commands. A normal render creates intermediate files under `.out/`; those files should not be committed.

## Making a change

1. Search for existing tests and follow the package’s established style.
2. Keep Go source formatted with `gofmt`.
3. Add or update focused tests for behavior changes, especially project validation, layout, or command generation.
4. Update `README.md` and/or `docs/` when a CLI option or YAML-facing behavior changes.
5. Run `go vet ./...` and `go test ./...` before opening a pull request.

Avoid bundling unrelated refactors with feature or bug-fix changes.

## Pull requests

Use the pull-request template to describe the problem, solution, tests, and documentation impact. Keep commits and the PR title clear enough to describe the user-visible change. Maintainers may ask for a smaller scope, tests, or documentation before merging.

## Reporting security issues

Do not post potential security vulnerabilities in public issues. Contact the repository maintainers privately through the contact method listed on the project’s GitHub profile.
