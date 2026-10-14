# YAML Format and Rendering Reference

This document describes the public YAML format and its behavior in yaml2video. The [video schema](video.schema.json) and [template schema](template.schema.json) define the exact allowed fields and constraints; the [user guide](user-guide.md) provides authoring examples.

## Documents and validation

A video supplies content and media; a template supplies design, composition, and output dimensions. Both require `$schema` and `version: 2`. Unsupported versions and unknown properties are rejected.

The CLI takes both files explicitly:

```sh
yaml2video -n -t template.yaml video.yaml  # validate and preview
yaml2video -t template.yaml video.yaml     # render
```

`video.template` must equal `template.template_id`. It is an identity check, not automatic template lookup. `$schema` is an editor reference; yaml2video validates against its bundled schemas rather than downloading that URL.

Schema validation checks structure. Composition and render-plan validation additionally check bindings, media paths, timing, and supported effect combinations. Passing schema validation alone does not guarantee a successful render.

## Video document

| Field | Requirement and purpose |
| --- | --- |
| `template` | Required template identifier. |
| `metadata` | Required `title` and `language`; optional `audience`, `description`, unique `tags`, and informational `target_duration_seconds`. |
| `assets` | Optional `logo` path and background `music` track. |
| `content` | Required nonempty `slides` array; optional `intro` and `outro`. |

Content fields:

| Item | Required | Optional |
| --- | --- | --- |
| `intro` | At least `title` or `image` | `title`, `image`, `subtitle`, `duration`, `audio`, `sound_effects` |
| slide | `image`, `text` | `id`, `label`, `tip`, `alt_text`, `duration`, `audio`, `sound_effects` |
| `outro` | `cta` | `image`, `title`, `text`, `duration`, `audio`, `sound_effects` |

The video cannot specify layout layers, typography, visual effects, codecs, or FFmpeg filters. Slide `alt_text` is retained in the render plan and shown in CLI dry-run output; it is not burned into the video.

## Template document

Besides `$schema` and `version`, every template requires:

| Field | Purpose |
| --- | --- |
| `template_id`, `description` | Identity and human-readable purpose. |
| `format` | `aspect_ratio`, `width`, `height`, and `fps`. Ratios: `9:16`, `1:1`, `16:9`, `4:5`. |
| `defaults` | Required `theme` and `timing`; optional `typography`, `media`, and `audio`. |
| `composition` | Ordered `sections` array, with at least two entries. |
| `layout` | Required `intro`, `slide`, and `outro` layouts, even when optional content is absent. |

- `theme`: required `background`, `text_color`, `accent_color`; optional `muted_color`, `panel_color`.
- `timing`: required `intro_duration`, `slide_duration`, `outro_duration`, and `transition`.
- `typography`: optional `font_family`, `title_size`, `body_size`, `tip_size`.
- `media`: optional `default_fit` and `default_position`; explicit element placement remains required by the schema.
- `audio`: optional `default_volume`, `fade_in`, `fade_out`, `normalize`, and `ducking`.

## Composition and bindings

Each section requires a unique `id`, a `role`, and its matching `source`:

| Role | Source | Behavior |
| --- | --- | --- |
| `intro` | `content.intro` | One scene if intro content exists; otherwise skipped. |
| `slide` | `content.slides` | Requires `repeat: true`; one scene per slide, in array order. |
| `outro` | `content.outro` | One scene if outro content exists; otherwise skipped. |

Composition must include a slide section. Optional section fields are `default_duration`, `duration_from: item.duration`, and `transition_out`. Intro and outro cannot repeat. Generated scene IDs must also be unique.

The renderer processes sections in order, binds content to the corresponding role layout, resolves defaults, and builds the scene timeline:

- Text elements bind to content fields with the same name; absent content or layout elements are omitted. `visible: false` hides an element.
- `logo` binds to `assets.logo` when the layout includes it.
- Slide `index` is generated as `current / total`; `progress` displays the slide's position in the sequence.
- Intro and outro images render as full-canvas backgrounds; their schemas do not expose a separate image layout element.

Content-level timing and audio values override template defaults. Scene duration uses the **first available** value:

```text
item.duration → section.default_duration → defaults.timing.<role>_duration
```

The current renderer honors `item.duration` whether or not `duration_from` is explicitly present.

## Timing and transitions

Durations are positive seconds, written as numbers (`3.5`) or strings (`3.5s`). Minutes and millisecond suffixes are unsupported. Sound-effect offsets may be zero.

Each scene uses its section's `transition_out`, falling back to `defaults.timing.transition`. For repeated slides, this also applies between slides. The last rendered scene has no outgoing transition; an explicit `transition_out` on the final composition section is rejected.

Supported transition types:

```text
cut, fade, dissolve,
wipe-left, wipe-right, wipe-up, wipe-down,
slide-left, slide-right, slide-up, slide-down,
circle-open, circle-close, radial, pixelize,
zoom-in, cover, reveal, blur
```

Non-cut transitions overlap adjacent scenes and must be strictly shorter than both. A `cut` has no overlap; its schema-required positive `duration` is ignored.

```text
next scene start = current scene end - outgoing overlap
effective duration = sum(scene durations) - sum(overlaps)
```

## Layout and visual effects

Every layout has a solid `background`. Required elements are intro `title`, slide `image` and `text`, and outro `cta`. Additional elements are defined in the template schema.

Elements require `placement.anchor`, optionally with pixel offsets `offset_x` and `offset_y`. Anchors are `top-left`, `top-center`, `top-right`, `center-left`, `center`, `center-right`, `bottom-left`, `bottom-center`, and `bottom-right`.

Text supports font family, size, normal/bold weight, color, alignment, maximum width, and maximum line count. Text exceeding `max_lines` is rejected, not truncated. The schema also permits `weight: medium` and `line_spacing`, but the current renderer rejects medium weight and nonzero line spacing. Media supports `cover`/`contain`, maximum width/height ratios, and `rectangle`, `rounded-rectangle`, or `circle` shapes.

An element's `effects` array specifies semantic effects, not arbitrary filter expressions:

| Effect | Element | Parameters |
| --- | --- | --- |
| `text_shadow` | Text | `color`, `opacity`, `offset_x`, `offset_y`, `blur` |
| `text_glow` | Text | `color`, `opacity`, `blur` |
| `text_outline` | Text | `color`, `opacity`, `width` |
| `glass_panel` | Text | `color`, `opacity`, `blur`, `padding` |
| `pan_zoom` | Image | `from_scale`, `to_scale`, `from_anchor`, `to_anchor` |
| `color_filter` | Image | `preset`, `intensity` |
| `vignette` | Image | `color`, `opacity` |
| `blur` | Image | `radius` |
| `fade` | Text or image | `fade_in`, `fade_out` |

Image filters follow array order. Text effects are implemented through ASS styling and compositing, not a general-purpose sequential filter graph. Color presets are `grayscale`, `sepia`, `warm`, `cool`, and `high-contrast`. A glass panel tints the text area, adds padding, and optionally blurs the background behind it.

Current restrictions include black-only vignettes, at most one glass panel per text element, and no text fade combined with a glass panel. Effects attached to the wrong element type and fades longer than the scene are rejected. Schema-valid colors may still be unsupported by the text renderer.

## Audio

Music, foreground `audio`, and `sound_effects` use a required `path` and optional `volume`, `fade_in`, and `fade_out`. Sound effects additionally require `at`, relative to their scene's start.

- Track settings override `defaults.audio`; volume is between `0` and `1`, falling back to `1`. Unspecified fades are absent.
- Background music loops or trims to the complete effective timeline. `normalize` controls music loudness normalization and defaults to enabled.
- Foreground audio begins with its scene. Sound effects start at `at` and mix with foreground audio and music.
- Sound effects must start before the scene ends. Audio fades must fit within the remaining scene duration, or the full video duration for music.
- `ducking.enabled: true` reduces music while foreground audio is active; sound effects do not trigger ducking. Defaults are `amount: 0.65`, `attack: 0.15s`, and `release: 0.4s`. The FFmpeg backend accepts attack from `0.00001s` to `2s` and release from `0.00001s` to `9s`.

## Paths, output, and dry-run limits

The v2 loader accepts local files only and rejects URI paths. Relative media paths resolve from the video YAML's directory. The API instead supplies a media base directory; see [server storage and media provisioning](server-api.md).

Media paths are checked for readability and regular-file status during loading. Decodability is left to FFmpeg; these checks do not guarantee a supported media format.

The CLI writes `output.mp4` next to the video YAML unless `-o` overrides it. Intermediate files go into `.yaml2video` beside the YAML unless `-work-dir` overrides it. Dimensions and frame rate come from the template; codecs and filter graphs are implementation details, not YAML fields.

`-n` reports output settings, effective duration, scene start/end times, text/media bindings, effects, audio events, and generated FFmpeg commands without encoding. It does not verify media decoding, installed fonts, or every FFmpeg capability. Actual text rendering requires an FFmpeg build with the `ass` filter and suitable fonts.

Unknown fields are not an extension mechanism: new format fields require schema and renderer support. Named output profiles and automatic template discovery are not current features.
