# Examples

Ready-to-render videos: office-mobility clips with German text, and an English promo for yaml2video itself. Each folder contains a `video.yaml` for content, a matching `template.yaml` for design, and an `assets/` folder with placeholder images and synthetic audio—not real exercise photography or narration.

All templates produce vertical **1080 × 1920 videos at 30 fps**.

| Example | What it demonstrates |
| --- | --- |
| [simple](simple/) | Three slides, basic text and images, fade transitions, and background music. Start here. |
| [normal](normal/) | Four slides, branded labels, rounded image cards, pan/zoom, and glass-panel text effects. |
| [complex](complex/) | Six slides, slide numbering, a sound effect, foreground audio, and music ducking. |
| [layouts](layouts/) | [Named layouts](../docs/layouts.md): a classic default, a classic variant, custom element-list layouts filled from `variables`, a custom intro, and an aliased outro. |
| [advanced](advanced/) | Every newer feature together. **Layouts:** a section-chosen custom intro, an aliased default slide layout, a named classic variant, a custom stat card filled from `variables` (fallbacks, optional, progress, audio, and effect elements), and an aliased custom outro. **[Advanced mode](../docs/advanced.md):** keyframe and effect presets, layout and template tracks (attached by role, layout, or scene, with offsets and optional clips), element clips that animate, modify, and hide layout elements, rotating and custom transitions with separate in/out sides, a ducking audio clip with volume keyframes, and a master bus. The template carries nearly everything; the video only adds a scene's transition override, `disable_tracks`, and one scene-specific track. |
| [showcase](showcase/) | A 36.5-second promo for yaml2video, made with yaml2video. Its template is a set of [scene templates](../docs/layouts.md#scene-templates) with their own length, transitions, motion, and sound, timed to a 120 BPM music bed; the video only picks scenes and fills variables. Shows [folders](../docs/advanced.md#folders), [text reveal](../docs/advanced.md#text-reveal) typing, syntax colors and a cursor from [match and replace](../docs/advanced.md#match-and-replace), counters, easings, effect layers, and custom transitions. Needs the fonts Clash Display (Variable), Fira Code, and Inter. |

## Run with Docker

From the **repository root**, build the image, check the simple example, then render it:

```sh
make build
make run ARGS="-n examples/simple/video.yaml"
make run ARGS="examples/simple/video.yaml"
```

`-n` validates the YAML and previews the render plan and FFmpeg commands without creating a video. Remove it to render.

Try the other examples:

```sh
make run ARGS="-t examples/normal/template.yaml examples/normal/video.yaml"
make run ARGS="-t examples/complex/template.yaml examples/complex/video.yaml"
```

`make run` mounts the repository at `/data` and passes `ARGS` to the CLI. Each render writes `output.mp4` into its example folder, accessible on your computer. Intermediate files go into that folder's `.yaml2video/`. Add `-o my-video.mp4` inside `ARGS` to choose another output location.

## Run the binary directly

Without Docker, install yaml2video, FFmpeg with the `ass` filter, and suitable fonts. From the repository root:

```sh
./yaml2video -n examples/simple/video.yaml
./yaml2video examples/simple/video.yaml
./yaml2video -t examples/normal/template.yaml examples/normal/video.yaml
./yaml2video -t examples/complex/template.yaml examples/complex/video.yaml
```

The simple example links to its template with `template_file`, so `-t` is optional. An explicit `-t` can still select a template.

## Adapt an example

1. Copy an example folder, including its assets.
2. Edit `video.yaml` to change text, media, durations, or audio.
3. Edit `template.yaml` to change colors, typography, layout, or transitions.
4. Keep the video's `template` value equal to the template's `template_id`.
5. Run with `-n` first, then render.

Media paths are relative to the **video YAML file**, not the shell's working directory. If you move the folder outside `examples/`, also update its `$schema` references for your editor.

See the [user guide](../docs/user-guide.md) for authoring instructions and the [format and rendering reference](../docs/reference.md) for supported fields and renderer limits.
