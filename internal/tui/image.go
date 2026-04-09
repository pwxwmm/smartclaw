package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type ImagePreview struct {
	path    string
	width   int
	height  int
	ascii   []string
	info    string
	visible bool
	theme   Theme
}

func NewImagePreview(width, height int) *ImagePreview {
	return &ImagePreview{
		width:   width,
		height:  height,
		visible: false,
		theme:   GetTheme(),
	}
}

func (i *ImagePreview) SetImage(path string, ascii []string, info string) {
	i.path = path
	i.ascii = ascii
	i.info = info
	i.visible = true
}

func (i *ImagePreview) Clear() {
	i.path = ""
	i.ascii = make([]string, 0)
	i.info = ""
	i.visible = false
}

func (i *ImagePreview) IsVisible() bool {
	return i.visible
}

func (i *ImagePreview) Render() string {
	if !i.visible {
		return ""
	}

	var sb strings.Builder

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(i.theme.Border).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(i.theme.Title)

	infoStyle := lipgloss.NewStyle().
		Foreground(i.theme.TextMuted)

	sb.WriteString(titleStyle.Render("📷 Image Preview"))
	sb.WriteString("\n\n")

	if len(i.ascii) > 0 {
		asciiArt := strings.Join(i.ascii, "\n")
		sb.WriteString(asciiArt)
		sb.WriteString("\n\n")
	}

	if i.path != "" {
		sb.WriteString(infoStyle.Render("File: " + i.path))
		sb.WriteString("\n")
	}

	if i.info != "" {
		sb.WriteString(infoStyle.Render(i.info))
	}

	return boxStyle.Render(sb.String())
}

func (i *ImagePreview) RenderThumbnail() string {
	if !i.visible {
		return ""
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(i.theme.Border).
		Width(i.width).
		Height(i.height).
		Padding(0, 1)

	if len(i.ascii) > 0 {
		maxLines := i.height - 2
		if len(i.ascii) > maxLines {
			return boxStyle.Render(strings.Join(i.ascii[:maxLines], "\n"))
		}
		return boxStyle.Render(strings.Join(i.ascii, "\n"))
	}

	return boxStyle.Render(fmt.Sprintf("📷 %s", i.path))
}

type ImageGallery struct {
	images   []ImagePreview
	selected int
	visible  bool
	width    int
	height   int
}

func NewImageGallery(width, height int) *ImageGallery {
	return &ImageGallery{
		images:   make([]ImagePreview, 0),
		selected: 0,
		visible:  false,
		width:    width,
		height:   height,
	}
}

func (g *ImageGallery) AddImage(path string, ascii []string, info string) {
	img := NewImagePreview(g.width/3, g.height-4)
	img.SetImage(path, ascii, info)
	g.images = append(g.images, *img)
}

func (g *ImageGallery) Next() {
	if len(g.images) > 0 {
		g.selected = (g.selected + 1) % len(g.images)
	}
}

func (g *ImageGallery) Prev() {
	if len(g.images) > 0 {
		g.selected = (g.selected - 1 + len(g.images)) % len(g.images)
	}
}

func (g *ImageGallery) GetSelected() *ImagePreview {
	if g.selected >= 0 && g.selected < len(g.images) {
		return &g.images[g.selected]
	}
	return nil
}

func (g *ImageGallery) Clear() {
	g.images = make([]ImagePreview, 0)
	g.selected = 0
	g.visible = false
}

func (g *ImageGallery) Render() string {
	if !g.visible || len(g.images) == 0 {
		return ""
	}

	theme := GetTheme()

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary)

	statusStyle := lipgloss.NewStyle().
		Foreground(theme.TextMuted)

	var sb strings.Builder

	sb.WriteString(titleStyle.Render("🖼️ Image Gallery"))
	sb.WriteString(" ")
	sb.WriteString(statusStyle.Render(fmt.Sprintf("(%d/%d)", g.selected+1, len(g.images))))
	sb.WriteString("\n\n")

	selectedImg := g.GetSelected()
	if selectedImg != nil {
		sb.WriteString(selectedImg.Render())
	}

	sb.WriteString("\n")
	sb.WriteString(theme.HelpStyle().Render("← →: Navigate | Esc: Close"))

	return sb.String()
}

func (g *ImageGallery) RenderThumbnails() string {
	if !g.visible || len(g.images) == 0 {
		return ""
	}

	theme := GetTheme()

	var thumbnails []string
	for i := range g.images {
		if i == g.selected {
			thumbnails = append(thumbnails, theme.InfoStyle().Render("[●]"))
		} else {
			thumbnails = append(thumbnails, theme.HelpStyle().Render("[○]"))
		}
	}

	return strings.Join(thumbnails, " ")
}

func (g *ImageGallery) IsVisible() bool {
	return g.visible
}

func (g *ImageGallery) Show() {
	g.visible = true
}

func (g *ImageGallery) Hide() {
	g.visible = false
}

func (g *ImageGallery) HasImages() bool {
	return len(g.images) > 0
}

type ImageViewer struct {
	image   *ImagePreview
	gallery *ImageGallery
	mode    string
	visible bool
}

func NewImageViewer(width, height int) *ImageViewer {
	return &ImageViewer{
		image:   NewImagePreview(width, height),
		gallery: NewImageGallery(width, height),
		mode:    "single",
		visible: false,
	}
}

func (v *ImageViewer) ShowSingle(path string, ascii []string, info string) {
	v.image.SetImage(path, ascii, info)
	v.mode = "single"
	v.visible = true
}

func (v *ImageViewer) ShowGallery() {
	v.gallery.Show()
	v.mode = "gallery"
	v.visible = true
}

func (v *ImageViewer) Hide() {
	v.image.Clear()
	v.gallery.Hide()
	v.visible = false
}

func (v *ImageViewer) Next() {
	if v.mode == "gallery" {
		v.gallery.Next()
	}
}

func (v *ImageViewer) Prev() {
	if v.mode == "gallery" {
		v.gallery.Prev()
	}
}

func (v *ImageViewer) Render() string {
	if !v.visible {
		return ""
	}

	switch v.mode {
	case "single":
		return v.image.Render()
	case "gallery":
		return v.gallery.Render()
	default:
		return ""
	}
}

func (v *ImageViewer) IsVisible() bool {
	return v.visible
}

func (v *ImageViewer) GetMode() string {
	return v.mode
}
