package render

import (
	"fmt"
	"math"
	"path/filepath"
	"time"

	"github.com/ondics/yaml2video/project"
	"github.com/ondics/yaml2video/types"
)

const (
	defaultWorkDir = ".out"
	defaultOutput  = "output.mp4"
	defaultFit     = "cover"
)

// Options controls where compilation writes its intermediate and final files.
type Options struct {
	WorkDir string
	Output  string
}

type pixelBounds struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Compile validates a project and converts it into an FFmpeg render plan.
func Compile(p *project.Project, options Options) (*Plan, error) {
	if err := Validate(p); err != nil {
		return nil, err
	}

	plan := newPlan(p, options)
	start := time.Duration(0)

	for index, scene := range p.Scenes {
		compiled, err := compileScene(p, scene, index, plan.WorkDir, start)
		if err != nil {
			return nil, fmt.Errorf("scene %q: %w", scene.ID, err)
		}

		plan.Scenes = append(plan.Scenes, compiled)
		start += compiled.Duration
	}

	addBoundaryTransitions(plan, p.Scenes)
	plan.Duration += start
	updateSceneStarts(plan)

	return plan, nil
}

func newPlan(p *project.Project, options Options) *Plan {
	background, _ := p.Video.Background.FFmpeg()

	workDir := options.WorkDir
	if workDir == "" {
		workDir = defaultWorkDir
	}

	output := options.Output
	if output == "" {
		output = defaultOutput
	}

	plan := &Plan{
		Video: VideoSpec{
			Width:      p.Video.Width,
			Height:     p.Video.Height,
			FPS:        p.Video.FPS,
			Background: background,
		},
		WorkDir: workDir,
		Output:  output,
	}

	if p.Music.Path != "" {
		plan.Music = &MusicPlan{
			Path:    p.Music.Path,
			Volume:  p.Music.Volume,
			FadeOut: seconds(p.Music.FadeOut),
		}
	}

	return plan
}

func compileScene(p *project.Project, scene project.Scene, index int, workDir string, start time.Duration) (ScenePlan, error) {
	duration := seconds(*scene.Duration)
	background := sceneBackground(p.Video.Background, scene.Background)
	assPath, outputPath := sceneArtifactPaths(workDir, index, scene.ID)

	compiled := ScenePlan{
		ID:         scene.ID,
		Start:      start,
		Duration:   duration,
		Background: background,
		ASSPath:    assPath,
		Output:     outputPath,
	}

	for _, layer := range scene.Layers {
		layerPlan, err := compileLayer(p, layer)
		if err != nil {
			return ScenePlan{}, err
		}

		if layer.Type == project.LayerTypeAudio {
			compiled.Audio = append(compiled.Audio, compileAudioLayer(layer, layerPlan))
			continue
		}

		compiled.Layers = append(compiled.Layers, layerPlan)
	}

	subtitles, err := compileSubtitles(p, scene.Subtitles)
	if err != nil {
		return ScenePlan{}, err
	}

	compiled.Subtitles = subtitles
	compiled.ASS = assDocument(p.Video.Width, p.Video.Height, compiled)

	return compiled, nil
}

func sceneBackground(defaultColor types.Color, override *types.Color) string {
	color := defaultColor
	if override != nil {
		color = *override
	}

	background, _ := color.FFmpeg()
	return background
}

func sceneArtifactPaths(workDir string, index int, sceneID string) (assPath, outputPath string) {
	filename := fmt.Sprintf("%03d-%s", index, safeName(sceneID))

	return filepath.Join(workDir, "ass", filename+".ass"),
		filepath.Join(workDir, "scenes", filename+".mkv")
}

func compileAudioLayer(layer project.Layer, plan LayerPlan) AudioLayerPlan {
	volume := 1.0
	if layer.Volume != nil {
		volume = *layer.Volume
	}

	return AudioLayerPlan{
		Path:         plan.Path,
		Volume:       volume,
		SourceOffset: plan.SourceOffset,
		Duration:     plan.Duration,
	}
}

func compileSubtitles(p *project.Project, subtitles []project.Subtitle) ([]SubtitlePlan, error) {
	background, err := compileTextBackground(p.Defaults.Subtitle.Background)
	if err != nil {
		return nil, err
	}

	plans := make([]SubtitlePlan, 0, len(subtitles))
	offset := time.Duration(0)

	for _, subtitle := range subtitles {
		if subtitle.Offset != nil {
			offset = seconds(*subtitle.Offset)
		}

		spans, err := compileTextSpans(subtitle.Text, p.Defaults.Subtitle.Color)
		if err != nil {
			return nil, err
		}

		end := offset + seconds(subtitle.Duration)
		plans = append(plans, SubtitlePlan{
			Spans:      spans,
			Font:       p.Defaults.Subtitle.Font,
			FontSize:   p.Defaults.Subtitle.Size,
			Color:      colorString(p.Defaults.Subtitle.Color),
			X:          p.Video.Width / 2,
			Y:          p.Video.Height - 16,
			Start:      offset,
			End:        end,
			In:         compileTransition(subtitle.Transition, true),
			Out:        compileTransition(subtitle.Transition, false),
			Background: background,
		})

		offset = end
	}

	return plans, nil
}

func addBoundaryTransitions(plan *Plan, scenes []project.Scene) {
	for index := 0; index < len(scenes)-1; index++ {
		transition := compileTransition(scenes[index].Transition, false)
		if transition == nil {
			continue
		}

		plan.Transitions = append(plan.Transitions, BoundaryTransition{
			FromScene: index,
			Type:      transition.Type,
			Duration:  transition.Duration,
			Offset:    plan.Scenes[index].Start + plan.Scenes[index].Duration - transition.Duration,
		})
		plan.Duration -= transition.Duration
	}
}

// Validate verifies that the project can be compiled into a render plan.
func Validate(p *project.Project) error {
	if p == nil {
		return fmt.Errorf("project is required")
	}
	if p.Video.Width <= 0 || p.Video.Height <= 0 || p.Video.FPS <= 0 {
		return fmt.Errorf("video width, height, and fps must be positive")
	}
	if _, err := p.Video.Background.FFmpeg(); err != nil {
		return fmt.Errorf("video background: %w", err)
	}
	if len(p.Scenes) == 0 {
		return fmt.Errorf("at least one scene is required")
	}

	seenIDs := make(map[string]bool, len(p.Scenes))
	for index, scene := range p.Scenes {
		if err := validateScene(scene, index, seenIDs); err != nil {
			return err
		}
	}

	return nil
}

func validateScene(scene project.Scene, index int, seenIDs map[string]bool) error {
	if scene.ID == "" {
		return fmt.Errorf("scene %d: id is required", index)
	}
	if seenIDs[scene.ID] {
		return fmt.Errorf("scene %q: id must be unique", scene.ID)
	}
	seenIDs[scene.ID] = true

	if scene.Duration == nil || *scene.Duration <= 0 {
		return fmt.Errorf("scene %q: positive duration is required", scene.ID)
	}
	if scene.Background != nil {
		if _, err := scene.Background.FFmpeg(); err != nil {
			return fmt.Errorf("scene %q background: %w", scene.ID, err)
		}
	}
	if err := validateTransitions(scene.Transition, *scene.Duration, "scene "+scene.ID); err != nil {
		return err
	}
	if scene.Transition != nil && scene.Transition.In != nil {
		return fmt.Errorf("scene %q: only transition.out is valid; it defines the boundary to the following scene", scene.ID)
	}

	for _, layer := range scene.Layers {
		if err := validateMediaLayer(scene.ID, *scene.Duration, layer); err != nil {
			return err
		}
	}

	return validateSubtitles(scene.ID, *scene.Duration, scene.Subtitles)
}

func validateMediaLayer(sceneID string, sceneDuration float64, layer project.Layer) error {
	if layer.Type != project.LayerTypeVideo && layer.Type != project.LayerTypeAudio {
		return nil
	}
	if layer.SourceOffset < 0 {
		return fmt.Errorf("scene %q: %s source_offset must not be negative", sceneID, layer.Type)
	}
	if layer.Duration != nil && (*layer.Duration <= 0 || *layer.Duration > sceneDuration) {
		return fmt.Errorf("scene %q: %s duration must be positive and no longer than the scene", sceneID, layer.Type)
	}

	return nil
}

func validateSubtitles(sceneID string, sceneDuration float64, subtitles []project.Subtitle) error {
	offset := 0.0

	for _, subtitle := range subtitles {
		if subtitle.Duration <= 0 {
			return fmt.Errorf("scene %q: subtitle duration must be positive", sceneID)
		}
		if subtitle.Offset != nil {
			offset = *subtitle.Offset
		}
		if offset+subtitle.Duration > sceneDuration {
			return fmt.Errorf("scene %q: subtitle extends beyond the scene duration", sceneID)
		}

		offset += subtitle.Duration
		if err := validateTransitions(subtitle.Transition, subtitle.Duration, "subtitle"); err != nil {
			return err
		}
	}

	return nil
}

func validateTransitions(values *project.SceneTransition, maximum float64, owner string) error {
	if values == nil {
		return nil
	}

	for _, transition := range []*project.Transition{values.In, values.Out} {
		if transition == nil {
			continue
		}
		if transition.Duration <= 0 || transition.Duration > maximum {
			return fmt.Errorf("%s transition duration must be within its visible duration", owner)
		}
		if transition.Type != "fade" && transition.Type != "dissolve" {
			return fmt.Errorf("%s transition type %q is unsupported", owner, transition.Type)
		}
		if owner == "subtitle" && transition.Type != "fade" {
			return fmt.Errorf("subtitle transition type %q is unsupported; subtitles currently support fade", transition.Type)
		}
	}

	return nil
}

func compileLayer(p *project.Project, layer project.Layer) (LayerPlan, error) {
	opacity, err := layerOpacity(layer)
	if err != nil {
		return LayerPlan{}, err
	}

	plan := LayerPlan{
		Kind:    string(layer.Type),
		Opacity: opacity,
	}
	if layer.Type == project.LayerTypeText {
		return compileTextLayer(p, layer, plan)
	}

	bounds, err := resolveTransform(p.Video.Width, p.Video.Height, layer.LayerLayout, layer.Transform)
	if err != nil {
		return LayerPlan{}, err
	}
	plan.X = bounds.X
	plan.Y = bounds.Y
	plan.Width = bounds.Width
	plan.Height = bounds.Height

	switch layer.Type {
	case project.LayerTypeImage, project.LayerTypeVideo:
		return compileMediaLayer(layer, plan)
	case project.LayerTypeRect, project.LayerTypeCircle:
		return compileShapeLayer(layer, plan)
	case project.LayerTypeAudio:
		return compileAudioSource(layer, plan)
	default:
		return plan, fmt.Errorf("unsupported layer type %q", layer.Type)
	}
}

func layerOpacity(layer project.Layer) (float64, error) {
	opacity := 1.0
	if layer.Opacity != nil {
		opacity = *layer.Opacity
	}
	if opacity < 0 || opacity > 1 {
		return 0, fmt.Errorf("layer opacity must be between 0 and 1")
	}

	return opacity, nil
}

func compileMediaLayer(layer project.Layer, plan LayerPlan) (LayerPlan, error) {
	if layer.Path == "" {
		return plan, fmt.Errorf("%s path is required", layer.Type)
	}

	fit := layer.Fit
	if fit == "" {
		fit = defaultFit
	}
	if fit != "cover" && fit != "contain" {
		return plan, fmt.Errorf("unsupported image fit %q", fit)
	}

	plan.Path = layer.Path
	plan.Fit = fit

	if layer.Type == project.LayerTypeVideo {
		plan.SourceOffset = seconds(layer.SourceOffset)
		if layer.Duration != nil {
			plan.Duration = seconds(*layer.Duration)
		}
		if layer.Trim != nil {
			if err := layer.Trim.Validate(); err != nil {
				return plan, fmt.Errorf("video trim: %w", err)
			}
			plan.Trim = &TrimPlan{
				Top:    layer.Trim.Top,
				Bottom: layer.Trim.Bottom,
				Left:   layer.Trim.Left,
				Right:  layer.Trim.Right,
			}
		}
	} else if layer.Trim != nil {
		return plan, fmt.Errorf("trim is only supported for video layers")
	}

	return plan, nil
}

func compileShapeLayer(layer project.Layer, plan LayerPlan) (LayerPlan, error) {
	if layer.Color == nil {
		return plan, fmt.Errorf("%s color is required", layer.Type)
	}

	color, err := layer.Color.FFmpeg()
	if err != nil {
		return plan, err
	}

	plan.Color = color
	return plan, nil
}

func compileAudioSource(layer project.Layer, plan LayerPlan) (LayerPlan, error) {
	if layer.Path == "" {
		return plan, fmt.Errorf("audio path is required")
	}
	if layer.Volume != nil && *layer.Volume < 0 {
		return plan, fmt.Errorf("audio volume must not be negative")
	}

	plan.Path = layer.Path
	plan.SourceOffset = seconds(layer.SourceOffset)
	if layer.Duration != nil {
		plan.Duration = seconds(*layer.Duration)
	}

	return plan, nil
}

func compileTextLayer(p *project.Project, layer project.Layer, plan LayerPlan) (LayerPlan, error) {
	plan.Font = p.Defaults.Text.Font
	plan.FontSize = p.Defaults.Text.Size
	if layer.Size > 0 {
		plan.FontSize = layer.Size
	}
	plan.Color = colorString(p.Defaults.Text.Color)

	background := p.Defaults.Text.Background
	if layer.Background != nil {
		background = *layer.Background
	}

	compiledBackground, err := compileTextBackground(background)
	if err != nil {
		return plan, err
	}
	plan.Background = compiledBackground

	spans, err := compileTextSpans(layer.Text, p.Defaults.Text.Color)
	if err != nil {
		return plan, err
	}
	if len(spans) == 0 {
		return plan, fmt.Errorf("text content is required")
	}

	metrics := measureText(spans, plan.FontSize)
	layout := textLayout(layer.LayerLayout, metrics)
	bounds, err := resolveTransform(p.Video.Width, p.Video.Height, layout, layer.Transform)
	if err != nil {
		return plan, err
	}

	plan.X = bounds.X
	plan.Y = bounds.Y
	plan.Width = bounds.Width
	plan.Height = bounds.Height

	if layer.Width != nil {
		plan.WrapWidth = plan.Width
		spans = wrapTextSpans(spans, plan.WrapWidth, plan.FontSize)
		metrics = measureText(spans, plan.FontSize)

		if layer.Height == nil {
			layout = textLayout(layer.LayerLayout, metrics)
			bounds, err = resolveTransform(p.Video.Width, p.Video.Height, layout, layer.Transform)
			if err != nil {
				return plan, err
			}
			plan.X = bounds.X
			plan.Y = bounds.Y
			plan.Width = bounds.Width
			plan.Height = bounds.Height
		}
	}

	plan.Spans = spans
	plan.ItemBackgrounds = textItemBackgrounds(spans, metrics, plan)
	return plan, nil
}

// resolveTransform converts a transform from canvas-relative values to pixel bounds.
func resolveTransform(
	canvasWidth, canvasHeight int,
	layout project.LayerLayout,
	transform *project.Transform,
) (pixelBounds, error) {
	bounds, err := layerBounds(canvasWidth, canvasHeight, layout)
	if err != nil {
		return pixelBounds{}, err
	}
	if transform == nil {
		return bounds, nil
	}

	if transform.Custom != nil {
		return resolveCustomTransform(canvasWidth, canvasHeight, layout, bounds, transform.Custom)
	}
	if transform.Preset == nil || !transform.Preset.Valid() {
		return pixelBounds{}, fmt.Errorf("invalid transform")
	}

	bounds.X = presetX(*transform.Preset, canvasWidth, bounds.Width)
	bounds.Y = presetY(*transform.Preset, canvasHeight, bounds.Height)
	return bounds, nil
}

func layerBounds(canvasWidth, canvasHeight int, layout project.LayerLayout) (pixelBounds, error) {
	bounds := pixelBounds{Width: canvasWidth, Height: canvasHeight}
	if layout.Width != nil {
		if *layout.Width <= 0 {
			return pixelBounds{}, fmt.Errorf("layer width must be positive")
		}
		bounds.Width = *layout.Width
	}
	if layout.Height != nil {
		if *layout.Height <= 0 {
			return pixelBounds{}, fmt.Errorf("layer height must be positive")
		}
		bounds.Height = *layout.Height
	}

	return bounds, nil
}

func resolveCustomTransform(
	canvasWidth, canvasHeight int,
	layout project.LayerLayout,
	bounds pixelBounds,
	transform *project.CustomTransform,
) (pixelBounds, error) {
	if transform.X < 0 || transform.X > 1 || transform.Y < 0 || transform.Y > 1 || transform.ScaleX <= 0 || transform.ScaleY <= 0 {
		return pixelBounds{}, fmt.Errorf("custom transform x/y must be in [0,1] and scales must be positive")
	}
	if transform.ScaleX != 1 && layout.Width == nil {
		return pixelBounds{}, fmt.Errorf("transform scale_x requires an explicit layer width")
	}
	if transform.ScaleY != 1 && layout.Height == nil {
		return pixelBounds{}, fmt.Errorf("transform scale_y requires an explicit layer height")
	}

	bounds.Width = int(math.Round(float64(bounds.Width) * transform.ScaleX))
	bounds.Height = int(math.Round(float64(bounds.Height) * transform.ScaleY))
	bounds.X = int(math.Round(float64(canvasWidth)*transform.X)) - bounds.Width/2
	bounds.Y = int(math.Round(float64(canvasHeight)*transform.Y)) - bounds.Height/2

	return bounds, nil
}

func presetX(preset project.TransformPreset, canvasWidth, width int) int {
	switch preset {
	case project.TransformPresetCenterLeft, project.TransformPresetTopLeft, project.TransformPresetBottomLeft:
		return 0
	case project.TransformPresetCenterRight, project.TransformPresetTopRight, project.TransformPresetBottomRight:
		return canvasWidth - width
	default:
		return (canvasWidth - width) / 2
	}
}

func presetY(preset project.TransformPreset, canvasHeight, height int) int {
	switch preset {
	case project.TransformPresetTopLeft, project.TransformPresetTopCenter, project.TransformPresetTopRight:
		return 0
	case project.TransformPresetBottomLeft, project.TransformPresetBottomCenter, project.TransformPresetBottomRight:
		return canvasHeight - height
	default:
		return (canvasHeight - height) / 2
	}
}

func updateSceneStarts(plan *Plan) {
	start := time.Duration(0)

	for index := range plan.Scenes {
		scene := &plan.Scenes[index]
		scene.Start = start
		start += scene.Duration

		if transition := boundaryFor(plan.Transitions, index); transition != nil {
			transition.Offset = start - transition.Duration
			start -= transition.Duration
		}
	}
}

func compileTransition(transitions *project.SceneTransition, in bool) *TransitionPlan {
	if transitions == nil {
		return nil
	}

	transition := transitions.Out
	if in {
		transition = transitions.In
	}
	if transition == nil {
		return nil
	}

	return &TransitionPlan{
		Type:     transition.Type,
		Duration: seconds(transition.Duration),
	}
}

func seconds(value float64) time.Duration {
	return time.Duration(math.Round(value * float64(time.Second)))
}

func compileTextSpans(items []project.TextItem, defaultColor types.Color) ([]TextSpan, error) {
	spans := make([]TextSpan, 0, len(items))
	for _, item := range items {
		if item.Short != nil {
			spans = append(spans, TextSpan{
				Content: string(*item.Short),
				Color:   colorString(defaultColor),
			})
		}
		if item.Long != nil {
			color := defaultColor
			if item.Long.Color != nil {
				color = *item.Long.Color
			}

			var itemBackground *TextBackgroundPlan
			if item.Long.Background != nil {
				compiledBackground, err := compileTextBackground(*item.Long.Background)
				if err != nil {
					return nil, err
				}
				itemBackground = compiledBackground
			}

			spans = append(spans, TextSpan{
				Content:    item.Long.Content,
				Color:      colorString(color),
				Bold:       item.Long.Bold,
				Italic:     item.Long.Italic,
				Underline:  item.Long.Underline,
				Background: itemBackground,
			})
		}
	}

	return spans, nil
}

func compileTextBackground(background project.TextBackground) (*TextBackgroundPlan, error) {
	if background.Color == "" {
		return nil, nil
	}
	opacity := 1.0
	if background.Opacity != nil {
		opacity = *background.Opacity
	}
	if opacity < 0 || opacity > 1 {
		return nil, fmt.Errorf("text background opacity must be between 0 and 1")
	}
	if background.Padding.X < 0 || background.Padding.Y < 0 {
		return nil, fmt.Errorf("text background padding must not be negative")
	}
	if background.Radius < 0 {
		return nil, fmt.Errorf("text background radius must not be negative")
	}

	color, err := background.Color.FFmpeg()
	if err != nil {
		return nil, fmt.Errorf("text background color: %w", err)
	}

	return &TextBackgroundPlan{
		Color:    color,
		Opacity:  opacity,
		PaddingX: background.Padding.X,
		PaddingY: background.Padding.Y,
		Radius:   background.Radius,
	}, nil
}
