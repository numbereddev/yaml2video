# Changelog

All notable changes to the project are documented here.

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
