package ui

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	glamourstyles "charm.land/glamour/v2/styles"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChromaHexLandsOnTheSameBasicColor(t *testing.T) {
	for i := range 16 {
		index := strconv.Itoa(i)
		t.Run(index, func(t *testing.T) {
			// given
			style := chroma.MustNewStyle("probe-"+index, chroma.StyleEntries{
				chroma.Keyword: chromaHex(index),
			})
			tokens := chroma.Literator(chroma.Token{Type: chroma.Keyword, Value: "x"})
			var out strings.Builder

			// when
			err := formatters.TTY16.Format(&out, style, tokens)

			// then
			require.NoError(t, err)
			expected := "\x1b[" + strconv.Itoa(30+i) + "m"
			if i >= 8 {
				expected = "\x1b[" + strconv.Itoa(90+i-8) + "m"
			}
			assert.Contains(t, out.String(), expected)
		})
	}
}

func TestChromaHexRejectsNonBasicIndexes(t *testing.T) {
	for _, index := range []string{"", "16", "-1", "abc", "#ff0000"} {
		t.Run(index, func(t *testing.T) {
			// when
			result := chromaHex(index)

			// then
			assert.Empty(t, result)
		})
	}
}

func TestMarkdownStyleUsesOnlyPaletteColors(t *testing.T) {
	// given
	p := defaultPalette
	allowed := map[string]bool{}
	for _, c := range []string{p.HeaderName, p.User, p.Footer, p.Error, p.Info, p.Activity, p.TurnSep, p.Telemetry} {
		if c != "" {
			allowed[c] = true
			allowed[chromaHex(c)] = true
		}
	}

	// when
	result := markdownStyle(p)

	// then
	for path, c := range colorFields(reflect.ValueOf(result), "StyleConfig") {
		if strings.HasSuffix(path, ".BackgroundColor") {
			assert.Failf(t, "background color set", "%s = %q", path, c)
			continue
		}
		assert.Truef(t, allowed[c], "%s = %q is not a palette color", path, c)
	}
}

func TestMarkdownStyleMapsRolesToSlots(t *testing.T) {
	// given
	p := defaultPalette

	// when
	result := markdownStyle(p)

	// then
	assert.Nil(t, result.Document.Color)
	assert.Equal(t, p.HeaderName, deref(result.Heading.Color))
	assert.Equal(t, p.Info, deref(result.LinkText.Color))
	assert.Equal(t, p.TurnSep, deref(result.Link.Color))
	assert.Equal(t, p.Info, deref(result.Code.Color))
	require.NotNil(t, result.CodeBlock.Chroma)
	assert.Equal(t, chromaHex(p.HeaderName), deref(result.CodeBlock.Chroma.Keyword.Color))
	assert.Equal(t, chromaHex(p.Info), deref(result.CodeBlock.Chroma.LiteralString.Color))
	assert.Equal(t, chromaHex(p.TurnSep), deref(result.CodeBlock.Chroma.Comment.Color))
	assert.Equal(t, chromaHex(p.Error), deref(result.CodeBlock.Chroma.Error.Color))
}

func TestMarkdownStyleKeepsTextAttributes(t *testing.T) {
	// when
	result := markdownStyle(defaultPalette)

	// then
	assert.True(t, deref(result.Emph.Italic))
	assert.True(t, deref(result.Strong.Bold))
	assert.True(t, deref(result.Strikethrough.CrossedOut))
	assert.True(t, deref(result.LinkText.Underline))
	assert.Empty(t, result.Emph.BlockPrefix)
	assert.Empty(t, result.Strong.BlockPrefix)
}

func TestMarkdownStyleLeavesGlamourDarkUntouched(t *testing.T) {
	// given
	dark := glamourstyles.DarkStyleConfig
	expectedHeading := deref(dark.Heading.Color)
	expectedCodeBackground := deref(dark.Code.BackgroundColor)
	expectedKeyword := deref(dark.CodeBlock.Chroma.Keyword.Color)

	// when
	markdownStyle(defaultPalette)

	// then
	assert.Equal(t, expectedHeading, deref(glamourstyles.DarkStyleConfig.Heading.Color))
	assert.Equal(t, expectedCodeBackground, deref(glamourstyles.DarkStyleConfig.Code.BackgroundColor))
	assert.Equal(t, expectedKeyword, deref(glamourstyles.DarkStyleConfig.CodeBlock.Chroma.Keyword.Color))
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func colorFields(v reflect.Value, path string) map[string]string {
	found := map[string]string{}
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			for k, c := range colorFields(v.Elem(), path) {
				found[k] = c
			}
		}
	case reflect.Struct:
		for i := range v.NumField() {
			f := v.Type().Field(i)
			fv := v.Field(i)
			name := path + "." + f.Name
			if (f.Name == "Color" || f.Name == "BackgroundColor") && fv.Kind() == reflect.Pointer {
				if !fv.IsNil() {
					found[name] = fv.Elem().String()
				}
				continue
			}
			for k, c := range colorFields(fv, name) {
				found[k] = c
			}
		}
	}
	return found
}
