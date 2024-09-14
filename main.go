package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/anthonynsimon/bild/transform"
	"golang.org/x/image/draw"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

type Section struct {
	Name   string
	EndPos float64
}

type Config struct {
	Font      string  `json:"font"`
	FontSize  float64 `json:"font_size"`
	BarColor  []uint8 `json:"bar_color"`
	TextColor []uint8 `json:"text_color"`
}

func readConfig() *Config {
	config := Config{}
	f, _ := os.Open("config.json")
	s, _ := io.ReadAll(f)
	_ = f.Close()
	_ = json.Unmarshal(s, &config)
	return &config
}

func loadFont(filename string, size float64) font.Face {
	f, _ := os.Open(filename)
	s, _ := io.ReadAll(f)
	_ = f.Close()
	fnt, _ := opentype.Parse(s)
	face, _ := opentype.NewFace(fnt, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingNone,
	})
	return face
}

func loadData(filename string) (width int, height int, barHeight int, rotation float64, sections []Section) {
	f, _ := os.Open(filename)
	scanner := bufio.NewScanner(f)
	width = 1920
	height = 1080
	barHeight = 100
	firstLine := true
	rotation = 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line[0] == '#' {
			continue
		}
		if firstLine {
			firstLine = false
			sl := strings.SplitN(line, " ", 4)
			width, _ = strconv.Atoi(sl[0])
			height, _ = strconv.Atoi(sl[1])
			barHeight, _ = strconv.Atoi(sl[2])
			rotation, _ = strconv.ParseFloat(sl[3], 64)
			continue
		}
		pos := strings.Index(line, " ")
		var tm string
		if pos == -1 {
			tm = line
		} else {
			tm = line[:pos]
		}
		t, err := time.Parse("15:04:05.999", tm)
		if err != nil {
			t, err = time.Parse("04:05.999", tm)
			if err != nil {
				fmt.Println("Error parsing time", tm)
				continue
			}
		}
		section := Section{}
		section.EndPos = float64(t.Minute()*60+t.Second()) + float64(t.Nanosecond())/1e9
		if pos > 0 {
			line = line[pos+1:]
			section.Name = line
		} else {
			section.Name = ""
		}
		sections = append(sections, section)
	}
	_ = f.Close()
	return
}

func main() {
	filename := "image.png"
	textfile := "text.png"
	dataname := "data.txt"
	configname := "config.json"
	flag.StringVar(&textfile, "t", "text.png", "Text image output filename")
	flag.StringVar(&dataname, "i", "data.txt", "Input filename")
	flag.StringVar(&configname, "c", "config.json", "Config filename")
	flag.Func("h", "Help", func(s string) error {
		fmt.Println("Usage: timeline-gen [options] [filename]")
		flag.PrintDefaults()
		os.Exit(0)
		return nil
	})
	flag.Parse()
	if flag.NArg() > 0 {
		filename = flag.Arg(0)
	}
	width, height, bottom, rotation, sections := loadData(dataname)
	if len(sections) == 0 {
		fmt.Println("No data found")
		return
	}
	timeMax := sections[len(sections)-1].EndPos
	config := readConfig()
	f, _ := os.Open(configname)
	s, _ := io.ReadAll(f)
	_ = f.Close()
	_ = json.Unmarshal(s, &config)

	upLeft := image.Point{}
	lowRight := image.Point{X: width, Y: height}

	img := image.NewRGBA(image.Rectangle{Min: upLeft, Max: lowRight})
	imgTxt := image.NewRGBA(image.Rectangle{Min: upLeft, Max: lowRight})

	face := loadFont(config.Font, config.FontSize)
	d := &font.Drawer{
		Dst:  imgTxt,
		Src:  image.NewUniform(color.RGBA{R: config.TextColor[0], G: config.TextColor[1], B: config.TextColor[2], A: config.TextColor[3]}),
		Face: face,
		Dot:  fixed.Point26_6{},
	}
	bgColor := color.RGBA{R: config.BarColor[0], G: config.BarColor[1], B: config.BarColor[2], A: config.BarColor[3]}
	bottomTop := height - bottom
	var start float64 = 0
	for _, section := range sections {
		startPos := start/timeMax*float64(width) + 2
		delimPos := section.EndPos/timeMax*float64(width) - 2
		for y := bottomTop; y < height; y++ {
			for x := int(startPos); x < int(delimPos); x++ {
				img.Set(x, y, bgColor)
			}
		}
		start = section.EndPos
	}
	start = 0
	alt := rotation < 0
	if alt {
		rotation = -rotation
	}
	for _, section := range sections {
		startPos := start/timeMax*float64(width) + 2
		delimPos := section.EndPos/timeMax*float64(width) - 2

		start = section.EndPos

		w := int(delimPos - startPos)
		rect, adv := d.BoundString(section.Name)
		var tw int
		if alt || rotation == 0 {
			tw = adv.Ceil()
			if rotation == 0 || tw <= w {
				d.Dot = fixed.Point26_6{X: fixed.I(int(startPos) + (w-tw)/2), Y: fixed.I(bottom/2-2+bottomTop) + (rect.Max.Y-rect.Min.Y)/2}
				d.DrawString(section.Name)
				continue
			}
		}
		tw = int(float64(adv.Ceil())*math.Cos(rotation*math.Pi/180) + float64((rect.Max.Y-rect.Min.Y).Ceil())*math.Sin(rotation*math.Pi/180) + 0.5)
		img2 := image.NewRGBA(image.Rectangle{Min: image.Point{}, Max: image.Point{X: adv.Ceil(), Y: (rect.Max.Y - rect.Min.Y).Ceil()}})
		d2 := &font.Drawer{
			Dst:  img2,
			Src:  image.NewUniform(color.RGBA{R: config.TextColor[0], G: config.TextColor[1], B: config.TextColor[2], A: config.TextColor[3]}),
			Face: face,
			Dot:  fixed.Point26_6{X: 0, Y: fixed.I((rect.Max.Y - rect.Min.Y).Ceil() - 2)},
		}
		d2.DrawString(section.Name)
		img2 = transform.Rotate(img2, -rotation, &transform.RotationOptions{ResizeBounds: true})
		offset := w/2 - tw/2
		if offset < 0 {
			offset = 0
		}
		minPt := image.Point{X: int(startPos) + offset, Y: height - img2.Rect.Dy()}
		maxPt := minPt.Add(img2.Rect.Size())
		draw.Draw(imgTxt, image.Rectangle{Min: minPt, Max: maxPt}, img2, image.Point{}, draw.Over)
	}
	_ = face.Close()

	// Encode as PNG.
	enc := png.Encoder{}
	enc.CompressionLevel = png.BestCompression

	of, _ := os.Create(filename)
	_ = enc.Encode(of, img)

	of, _ = os.Create(textfile)
	_ = enc.Encode(of, imgTxt)
}
