# Changelog

All notable changes to the project are documented here.

## Unreleased

### Added

- Named layouts (`docs/layouts.md`):
  - `layout:` holds any number of named layouts; `intro`, `slide`, and `outro` remain the role defaults, and a layout entry may be the name of another layout;
  - content items choose a layout with `layout:`, and template sections set a default for their scenes with `layout:`;
  - custom layouts are ordered element lists (`text`, `image`, `rect`, `circle`, `progress`, `audio`, `effect`) with no predefined fields; elements stack in list order and can be timed, keyframed, named with `id`, and marked `optional`;
  - `$(NAME)` variables (also `${NAME}` and `$(NAME|fallback)`) read the item's new `variables:` map, its fields, and generated values such as `$(index)` and `$(logo)`.
- Optional advanced mode (`advanced` in templates and videos); documents without it render unchanged:
  - keyframes with nine easings for position, scale, rotation, opacity, and audio volume;
  - template keyframes for standard layout elements (`animate`);
  - named keyframe and effect presets, shared between templates and videos;
  - tracks of image, shape, text, progress, audio, and effect clips, attached to the video, a role, a layout, or one scene, optionally delayed by a track `offset`, with `$(NAME)` variables;
  - `element` clips that animate or modify (`set`) a scene's layout elements in place, for a time window;
  - stackable effect layers: blur, color, color filters, vignette, glow, RGB split, grain, tint, shake, and keyframed camera moves;
  - a master bus that loudness-normalizes the final mix;
  - custom transitions (`advanced.transitions`): a built-in blend, a custom per-pixel expression, or a cut, with effect layers across the boundary, on the out side, or on the in side; plus per-scene overrides (`scene_transitions`).
- Transitions:
  - 41 more built-in xfade transitions (fade-black, smooth-*, slice-*, wind-*, squeeze-*, diag-*, cover-/reveal-* in all directions, and more);
  - `transition_out` lists rotate across a section's scenes, and the new `transition_in` sets transitions into a section's scenes; a transition given on one side covers both;
  - `defaults.timing.transition` is optional, and without any transition scenes cut; a transition's `duration` is optional in the schema (built-in blends still need one, custom transitions bring their own).
- Added the layouts and advanced examples, `docs/layouts.md`, and `docs/advanced.md`.

### Changed

- Video items no longer require `image`/`text` (slides), `title` or `image` (intro), or `cta` (outro) in the schema; the requirement now comes from the selected layout and is checked when loading.
- Track text renders at its position in the track rather than above all of the track's other clips.

### Fixed

- Reusing the same audio file several times in one video no longer fails to build the FFmpeg graph.
- `allOf`, `oneOf`, and `anyOf` schema failures now report the underlying error of the closest alternative; `if`/`then`/`else` is supported.
- A `cut` followed by a blended transition no longer fails with an xfade timebase mismatch.

## v2

### Added

- Introduced a two-document video format:
  - `video.yaml` contains localized video content, media assets, timing, and audio events.
  - `template.yaml` contains output format, composition, layout, typography, styling, effects, and defaults.
- Added JSON Schema Draft 2020-12 validation for both document types:
  - `docs/video.schema.json`
  - `docs/template.schema.json`
- Added three complete video examples for simple, standard, and complex office-mobility videos.
- Added three matching templates for vertical social-media videos.
- Added the semantic effect language for:
  - text shadows, glows, and outlines;
  - translucent and blurred glass panels;
  - image pan-and-zoom;
  - color filters, vignette, blur, and fade effects;
  - slide progress indicators.
- Added semantic slide transitions including fade, dissolve, wipe, slide, radial, pixelize, zoom, cover, reveal, and blur variants.
- Added section-level audio support for intro, slide, and outro content.
- Added multiple sound effects per section with relative start offsets.
- Added template-level audio normalization and optional background-music ducking for foreground audio.

### Breaking changes

- Old schemas and templates are no longer compatible with v2.
