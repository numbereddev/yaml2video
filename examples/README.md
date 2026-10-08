# v2 examples

Each of `simple/`, `normal/`, and `complex/` contains a `video-*.yaml` content document and its matching `template-*.yaml` layout document. These demonstrate progressively richer v2 composition, effects, transitions, and audio.

For example:

```sh
go run . -n -t examples/simple/template-simple.yaml examples/simple/video-simple.yaml
```

The v2 examples reference media under `assets/` that is **not included** in this repository. Provide your own files at those paths (or update the paths in the video YAML) before validating or rendering them. See the [v2 user guide](../docs/user-guide.md) for the format.
