package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	assScriptInfo = "[Script Info]\n" +
		"ScriptType: v4.00+\n" +
		"PlayResX: %d\n" +
		"PlayResY: %d\n\n"

	assStyles = "[V4+ Styles]\n" +
		"Format: Name,Fontname,Fontsize,PrimaryColour,SecondaryColour,OutlineColour,BackColour,Bold,Italic,Underline,StrikeOut,ScaleX,ScaleY,Spacing,Angle,BorderStyle,Outline,Shadow,Alignment,MarginL,MarginR,MarginV,Encoding\n" +
		"Style: Default,Arial,24,&H00FFFFFF,&H00FFFFFF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,3,0,0,5,0,0,0,1\n\n"

	assEvents = "[Events]\n" +
		"Format: Layer,Start,End,Style,Name,MarginL,MarginR,MarginV,Effect,Text\n"
)

func assDocument(width, height int, scene ScenePlan) string {
	var document strings.Builder

	fmt.Fprintf(&document, assScriptInfo, width, height)
	document.WriteString(assStyles)
	document.WriteString(assEvents)

	for _, layer := range scene.Layers {
		if layer.Kind != "text" {
			continue
		}

		text := assSpans(
			layer.Spans,
			layer.Font,
			layer.FontSize,
			layer.Color,
			layer.Opacity,
			nil,
			false,
		)
		tags, fadeIn, fadeOut := textEffectTags(layer.Effects)
		if tags != "" || fadeIn > 0 || fadeOut > 0 {
			prefix := fmt.Sprintf("{\\an5\\pos(%d,%d)%s", layer.X+layer.Width/2, layer.Y+layer.Height/2, tags)
			if fadeIn > 0 || fadeOut > 0 {
				prefix += fmt.Sprintf("\\fad(%d,%d)", fadeIn, fadeOut)
			}
			fmt.Fprintf(&document, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s}%s\n", assTime(0), assTime(scene.Duration), prefix, text)
		} else {
			writeASSDialogue(&document, 0, scene.Duration, 5, layer.X+layer.Width/2, layer.Y+layer.Height/2, text, nil, layer.Opacity, false, 0, 0)
		}
	}

	for _, subtitle := range scene.Subtitles {
		text := assSpans(
			subtitle.Spans,
			subtitle.Font,
			subtitle.FontSize,
			subtitle.Color,
			1,
			subtitle.Background,
			true,
		)
		fadeIn, fadeOut := subtitleFadeDurations(subtitle)

		writeASSDialogue(
			&document,
			subtitle.Start,
			subtitle.End,
			2,
			subtitle.X,
			subtitle.Y,
			text,
			subtitle.Background,
			1,
			true,
			fadeIn,
			fadeOut,
		)
	}

	return document.String()
}

func writeASSDialogue(
	document *strings.Builder,
	start, end time.Duration,
	alignment, x, y int,
	text string,
	background *TextBackgroundPlan,
	backgroundOpacity float64,
	includeFade bool,
	fadeIn, fadeOut int,
) {
	overrides := fmt.Sprintf("{\\an%d\\pos(%d,%d)", alignment, x, y)
	overrides += assBackground(background, backgroundOpacity)
	if includeFade {
		overrides += fmt.Sprintf("\\fad(%d,%d)", fadeIn, fadeOut)
	}
	overrides += "}"

	fmt.Fprintf(
		document,
		"Dialogue: 0,%s,%s,Default,,0,0,0,,%s%s\n",
		assTime(start),
		assTime(end),
		overrides,
		text,
	)
}

func subtitleFadeDurations(subtitle SubtitlePlan) (fadeIn, fadeOut int) {
	if subtitle.In != nil && subtitle.In.Type == "fade" {
		fadeIn = int(subtitle.In.Duration.Milliseconds())
	}
	if subtitle.Out != nil && subtitle.Out.Type == "fade" {
		fadeOut = int(subtitle.Out.Duration.Milliseconds())
	}

	return fadeIn, fadeOut
}

func assSpans(
	spans []TextSpan,
	font string,
	size int,
	defaultColor string,
	opacity float64,
	background *TextBackgroundPlan,
	includeItemBackground bool,
) string {
	var value strings.Builder
	activeBackground := background

	for _, span := range spans {
		color := span.Color
		if color == "" {
			color = defaultColor
		}

		spanBackground := background
		if includeItemBackground && span.Background != nil {
			spanBackground = span.Background
		}
		if !includeItemBackground {
			spanBackground = nil
		}

		backgroundTags := ""
		if spanBackground != activeBackground {
			backgroundTags = assBackground(spanBackground, opacity)
			activeBackground = spanBackground
		}

		fmt.Fprintf(
			&value,
			"{\\fn%s\\fs%d\\c%s\\1a%s\\b%d\\i%d\\u%d%s}%s",
			assEscape(font),
			size,
			assColor(color),
			assAlpha(color, opacity),
			boolInt(span.Bold),
			boolInt(span.Italic),
			boolInt(span.Underline),
			backgroundTags,
			assEscape(span.Content),
		)
	}

	return value.String()
}

func assBackground(background *TextBackgroundPlan, layerOpacity float64) string {
	if background == nil {
		return "\\xbord0\\ybord0\\3a&HFF&"
	}

	return fmt.Sprintf(
		"\\3c%s\\3a%s\\xbord%d\\ybord%d",
		assColor(background.Color),
		assAlpha(background.Color, background.Opacity*layerOpacity),
		background.PaddingX,
		background.PaddingY,
	)
}

func assColor(color string) string {
	value := strings.TrimPrefix(strings.Split(color, "@")[0], "0x")
	if len(value) != 6 {
		return "&HFFFFFF&"
	}

	return "&H00" + value[4:6] + value[2:4] + value[0:2] + "&"
}

func assAlpha(color string, opacity float64) string {
	alpha := 1.0
	if _, after, ok := strings.Cut(color, "@"); ok {
		alpha, _ = strconv.ParseFloat(after, 64)
	}
	alpha *= opacity

	return fmt.Sprintf("&H%02X&", int((1-alpha)*255+0.5))
}

func assEscape(value string) string {
	return strings.NewReplacer(
		"\\", "\\\\",
		"{", "\\{",
		"}", "\\}",
		"\n", "\\N",
	).Replace(value)
}

func assTime(value time.Duration) string {
	centiseconds := value.Milliseconds() / 10

	return fmt.Sprintf(
		"%d:%02d:%02d.%02d",
		centiseconds/360000,
		(centiseconds/6000)%60,
		(centiseconds/100)%60,
		centiseconds%100,
	)
}

func boolInt(value bool) int {
	if value {
		return 1
	}

	return 0
}

func v2ASSTime(d time.Duration) string {
	cs := int64(d / (10 * time.Millisecond))
	return fmt.Sprintf("%d:%02d:%02d.%02d", cs/360000, cs/6000%60, cs/100%60, cs%100)
}
func v2ASSAlpha(opacity float64) string     { return fmt.Sprintf("&H%02X&", int((1-opacity)*255+0.5)) }
func v2ASSColorSupported(color string) bool { return len(color) == 7 && strings.HasPrefix(color, "#") }
func v2ASSColor(color string) string {
	if strings.HasPrefix(color, "#") && len(color) == 7 {
		return "&H" + color[5:7] + color[3:5] + color[1:3] + "&"
	}
	return "&HFFFFFF&"
}

func v2EscapeASS(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "{", "\\{")
	s = strings.ReplaceAll(s, "}", "\\}")
	return strings.ReplaceAll(s, "\n", "\\N")
}

func v2ASSDocument(video VideoSpec, scene ScenePlan, alignments map[int]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Script Info]\nScriptType: v4.00+\nPlayResX: %d\nPlayResY: %d\n\n[V4+ Styles]\nFormat: Name,Fontname,Fontsize,PrimaryColour,SecondaryColour,OutlineColour,BackColour,Bold,Italic,Underline,StrikeOut,ScaleX,ScaleY,Spacing,Angle,BorderStyle,Outline,Shadow,Alignment,MarginL,MarginR,MarginV,Encoding\nStyle: Default,Arial,24,&H00FFFFFF,&H00FFFFFF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,3,0,0,5,0,0,0,1\n\n[Events]\nFormat: Layer,Start,End,Style,Name,MarginL,MarginR,MarginV,Effect,Text\n", video.Width, video.Height)
	for i, layer := range scene.Layers {
		if layer.Kind != "text" {
			continue
		}
		bold := 0
		if len(layer.Spans) > 0 && layer.Spans[0].Bold {
			bold = 1
		}
		font := strings.NewReplacer("{", "", "}", "", "\\", "").Replace(layer.Font)
		value := ""
		for _, span := range layer.Spans {
			value += v2EscapeASS(span.Content)
		}
		tags := ""
		fadeIn, fadeOut := 0, 0
		for _, fx := range layer.Effects {
			switch fx.Type {
			case "text_shadow":
				tags += fmt.Sprintf("\\4c%s\\4a%s\\xshad%g\\yshad%g\\blur%g", v2ASSColor(fx.Color), v2ASSAlpha(fx.Opacity), fx.OffsetX, fx.OffsetY, fx.Blur)
			case "text_outline", "text_glow":
				width := fx.Width
				if fx.Type == "text_glow" {
					width = math.Max(1, fx.Blur*2)
				}
				tags += fmt.Sprintf("\\3c%s\\3a%s\\bord%g\\blur%g", v2ASSColor(fx.Color), v2ASSAlpha(fx.Opacity), width, fx.Blur)
			case "fade":
				fadeIn, fadeOut = int(fx.FadeIn.Milliseconds()), int(fx.FadeOut.Milliseconds())
			}
		}
		if fadeIn > 0 || fadeOut > 0 {
			tags += fmt.Sprintf("\\fad(%d,%d)", fadeIn, fadeOut)
		}
		alignment, x := 5, layer.X+layer.Width/2
		switch alignments[i] {
		case "left":
			alignment, x = 4, layer.X
		case "right":
			alignment, x = 6, layer.X+layer.Width
		}
		fmt.Fprintf(&b, "Dialogue: 0,%s,%s,Default,,0,0,0,,{\\an%d\\pos(%d,%d)\\fn%s\\fs%d\\c%s\\b%d%s}%s\n", v2ASSTime(0), v2ASSTime(scene.Duration), alignment, x, layer.Y+layer.Height/2, font, layer.FontSize, v2ASSColor(layer.Color), bold, tags, value)
	}
	return b.String()
}
