package render

import (
	"math"
	"strings"
	"unicode"

	"github.com/ondics/yaml2video/src/project"
)

// wrapTextSpans wraps spans to width, measuring words with the font that
// draws them.
func wrapTextSpans(spans []TextSpan, width int, font string, fontSize int) []TextSpan {
	if width <= 0 || fontSize <= 0 {
		return spans
	}

	lineWidth := 0.0
	for index := range spans {
		measure := func(s string) float64 { return textWidthF(font, spans[index].Bold, s, fontSize) }
		spans[index].Content, lineWidth = wrapText(spans[index].Content, lineWidth, float64(width), measure)
	}

	return spans
}

func wrapText(value string, lineWidth, width float64, measure func(string) float64) (string, float64) {
	var result strings.Builder
	var pendingWhitespace string

	for _, token := range textTokens(value) {
		if token.isWhitespace {
			if strings.ContainsRune(token.value, '\n') {
				result.WriteByte('\n')
				lineWidth = 0
				pendingWhitespace = ""
			} else if lineWidth > 0 {
				pendingWhitespace = " "
			}
			continue
		}

		wordWidth := measure(token.value)
		space := 0.0
		if pendingWhitespace != "" {
			space = measure("a"+pendingWhitespace+"a") - measure("aa")
		}
		if lineWidth > 0 && lineWidth+space+wordWidth > width {
			result.WriteByte('\n')
			lineWidth = 0
			pendingWhitespace = ""
		}

		if pendingWhitespace != "" && lineWidth > 0 {
			result.WriteString(pendingWhitespace)
			lineWidth += space
		}

		result.WriteString(token.value)
		lineWidth += wordWidth
		pendingWhitespace = ""
	}

	return result.String(), lineWidth
}

type textToken struct {
	value        string
	isWhitespace bool
}

func textTokens(value string) []textToken {
	if value == "" {
		return nil
	}

	tokens := make([]textToken, 0)
	start := 0
	whitespace := false
	for index, character := range value {
		isWhitespace := unicode.IsSpace(character)
		if index == 0 {
			whitespace = isWhitespace
			continue
		}
		if isWhitespace != whitespace {
			tokens = append(tokens, textToken{value: value[start:index], isWhitespace: whitespace})
			start = index
			whitespace = isWhitespace
		}
	}
	tokens = append(tokens, textToken{value: value[start:], isWhitespace: whitespace})

	return tokens
}

type textMetrics struct {
	LineWidths []int
	Width      int
	Height     int
	LineHeight int
}

func textLayout(layout project.LayerLayout, metrics textMetrics) project.LayerLayout {
	result := layout
	if result.Width == nil {
		width := metrics.Width
		result.Width = &width
	}
	if result.Height == nil {
		height := metrics.Height
		result.Height = &height
	}

	return result
}

func measureText(spans []TextSpan, font string, fontSize int) textMetrics {
	lineHeight := max(int(math.Round(float64(fontSize)*1.2)), 1)

	lineWidths := []int{0}
	for _, span := range spans {
		lines := strings.Split(span.Content, "\n")
		for index, line := range lines {
			lineWidths[len(lineWidths)-1] += textWidth(font, span.Bold, line, fontSize)
			if index < len(lines)-1 {
				lineWidths = append(lineWidths, 0)
			}
		}
	}

	width := 1
	for _, lineWidth := range lineWidths {
		if lineWidth > width {
			width = lineWidth
		}
	}

	return textMetrics{
		LineWidths: lineWidths,
		Width:      width,
		Height:     len(lineWidths) * lineHeight,
		LineHeight: lineHeight,
	}
}

func textItemBackgrounds(spans []TextSpan, metrics textMetrics, layer LayerPlan) []TextBackgroundBox {
	boxes := make([]TextBackgroundBox, 0)
	line := 0
	lineOffset := 0
	textTop := layer.Y + (layer.Height-metrics.Height)/2

	for _, span := range spans {
		lines := strings.Split(span.Content, "\n")
		for index, value := range lines {
			width := textWidth(layer.Font, span.Bold, value, layer.FontSize)
			if span.Background != nil && width > 0 {
				lineStart := layer.X + (layer.Width-metrics.LineWidths[line])/2
				boxes = append(boxes, TextBackgroundBox{
					X:          lineStart + lineOffset - span.Background.PaddingX,
					Y:          textTop + line*metrics.LineHeight - span.Background.PaddingY,
					Width:      width + 2*span.Background.PaddingX,
					Height:     metrics.LineHeight + 2*span.Background.PaddingY,
					Background: *span.Background,
				})
			}

			lineOffset += width
			if index < len(lines)-1 {
				line++
				lineOffset = 0
			}
		}
	}

	return boxes
}

// textWidth is the width of value in font at fontSize, as libass draws it.
func textWidth(font string, bold bool, value string, fontSize int) int {
	width := int(math.Round(textWidthF(font, bold, value, fontSize)))
	if width < 1 && value != "" {
		return 1
	}

	return width
}

func textWidthF(font string, bold bool, value string, fontSize int) float64 {
	if value == "" {
		return 0
	}
	weight := 400
	if bold {
		weight = 700
	}
	x := defaultFonts.face(font, weight, 0).offsets([]rune(value), float64(fontSize), 0, 0)
	return x[len(x)-1]
}
