# Examples

Three ready-to-render office-mobility videos with German text. Each folder contains a `video-*.yaml` for content, a matching `template-*.yaml` for design, and an `assets/` folder with placeholder images and synthetic audio—not real exercise photography or narration.

All three templates produce vertical **1080 × 1920 videos at 30 fps**.

| Example | What it demonstrates |
| --- | --- |
| [simple](simple/) | Three slides, basic text and images, fade transitions, and background music. Start here. |
| [normal](normal/) | Four slides, branded labels, rounded image cards, pan/zoom, and glass-panel text effects. |
| [complex](complex/) | Six slides, slide numbering, a sound effect, foreground audio, and music ducking. |

## Run with Docker

From the **repository root**, build the image and open its shell:

```sh
make build
make bash
```

Inside the shell, the repository is mounted at `/data`. Check the simple example, then render it:

```sh
yaml2video -n -t examples/simple/template-simple.yaml examples/simple/video-simple.yaml
yaml2video -t examples/simple/template-simple.yaml examples/simple/video-simple.yaml
```

`-n` validates the YAML and previews the render plan and FFmpeg commands without creating a video. Remove it to render.

Try the other examples in the same shell:

```sh
yaml2video -t examples/normal/template-normal.yaml examples/normal/video-normal.yaml
yaml2video -t examples/complex/template-complex.yaml examples/complex/video-complex.yaml
```

Each render writes `output.mp4` into its example folder, accessible on your computer. Intermediate files go into that folder's `.yaml2video-v2/`. Add `-o /data/my-video.mp4` to choose another output location. Type `exit` to leave the shell.

Without Docker, install yaml2video, FFmpeg with the `ass` filter, and suitable fonts. Run the same commands from the repository root using the path to your executable (for example, `./yaml2video`).

## Adapt an example

1. Copy an example folder, including its assets.
2. Edit `video-*.yaml` to change text, media, durations, or audio.
3. Edit `template-*.yaml` to change colors, typography, layout, or transitions.
4. Keep the video's `template` value equal to the template's `template_id`.
5. Run with `-n` first, then render.

Media paths are relative to the **video YAML file**, not the shell's working directory. If you move the folder outside `examples/`, also update its `$schema` references for your editor.

See the [user guide](../docs/user-guide.md) for authoring instructions and the [format and rendering reference](../docs/reference.md) for supported fields and renderer limits.
