package main

// #cgo pkg-config: pdfium
// #include "fpdfview.h"
// #include "fpdf_annot.h"
// #include "fpdf_edit.h"
// #include "fpdf_structtree.h"
import "C"

import (
	"errors"
	"github.com/nfnt/resize"
	"go-pdfium-demo/render"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"math"
	"os"
	"sync"
	"unsafe"
)

// Document is good
type Document struct {
	doc C.FPDF_DOCUMENT
}

var wg sync.WaitGroup

// NewDocument shoud have docs
func NewDocument(data *[]byte) (*Document, error) {
	// doc := C.FPDF_LoadDocument(C.CString("in.pdf"), nil)
	doc := C.FPDF_LoadMemDocument(
		unsafe.Pointer(&((*data)[0])),
		C.int(len(*data)),
		nil)
	if doc == nil {
		defer C.FPDF_CloseDocument(doc)
		errorcase := C.FPDF_GetLastError()
		switch errorcase {
		case C.FPDF_ERR_SUCCESS:
			println("FPDF_ERR_SUCCESS")
		case C.FPDF_ERR_UNKNOWN:
			println("FPDF_ERR_UNKNOWN")
		case C.FPDF_ERR_FILE:
			println("FPDF_ERR_FILE")
		case C.FPDF_ERR_FORMAT:
			println("FPDF_ERR_FORMAT")
			println("Unknown error:", errorcase)
		case C.FPDF_ERR_PASSWORD:
			println("FPDF_ERR_PASSWORD")
		case C.FPDF_ERR_SECURITY:
			println("FPDF_ERR_SECURITY")
		case C.FPDF_ERR_PAGE:
			println("FPDF_ERR_PAGE")
		default:
			println("Unknown error:", errorcase)
		}
		return nil, errors.New("Error due to")
	}
	return &Document{doc: doc}, nil
}

// GetPageCount shoud have docs
func (d *Document) GetPageCount() int {
	return int(C.FPDF_GetPageCount(d.doc))
}

// CloseDocument shoud have docs
func (d *Document) CloseDocument() {
	C.FPDF_CloseDocument(d.doc)
}

// RenderPage shoud have docs
func (d *Document) RenderPage(i int, dpi int) *image.RGBA {
	page := C.FPDF_LoadPage(d.doc, C.int(i))
	//scale := dpi / 72.0
	//获取页面数据宽高
	//width := C.int(C.FPDF_GetPageWidth(page) * C.double(scale))
	//height := C.int(C.FPDF_GetPageHeight(page) * C.double(scale))
	width := C.int(C.FPDF_GetPageWidth(page))
	height := C.int(C.FPDF_GetPageHeight(page))
	alpha := C.FPDFPage_HasTransparency(page)
	//创建空白位图对象
	bitmap := C.FPDFBitmap_Create(width, height, alpha)
	fillColor := 4294967295
	if int(alpha) == 1 {
		fillColor = 0
	}
	C.FPDFBitmap_FillRect(bitmap, 0, 0, width, height, C.ulong(fillColor))
	// 渲染图片
	C.FPDF_RenderPageBitmap(bitmap, page, 0, 0, width, height, 0, C.FPDF_ANNOT)

	p := C.FPDFBitmap_GetBuffer(bitmap)
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	img.Stride = int(C.FPDFBitmap_GetStride(bitmap))

	bgra := make([]byte, 4)
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			for i := range bgra {
				bgra[i] = *((*byte)(p))
				p = unsafe.Pointer(uintptr(p) + 1)
			}
			img.SetRGBA(
				x, y,
				color.RGBA{
					B: bgra[0],
					G: bgra[1],
					R: bgra[2],
					A: bgra[3]})
		}
	}
	C.FPDFBitmap_Destroy(bitmap)
	C.FPDF_ClosePage(page)
	return img
}

func main() {
	//data, _ := ioutil.ReadFile(os.Args[1])
	//outImgName := "./outImg/"+os.Args[2]
	//C.FPDF_InitLibrary()
	////d, err := NewDocument(&data)
	//d, err := NewDocument(&data)
	//if err != nil {
	//	println(err)
	//} else {
	//	count := d.GetPageCount()
	//	img0 := d.RenderPage(0, 600)
	//	fb, _ := os.OpenFile(outImgName+".jpg", os.O_WRONLY|os.O_CREATE, 0600)
	//	jpeg.Encode(fb, img0, nil)
	//	fb.Close()
	//	imgSlice := make([]string, 0, 30)
	//	for i := 0; i < count; i++ {
	//
	//		img := d.RenderPage(i, 600)
	//		f, _ := os.OpenFile(outImgName+strconv.Itoa(i)+".jpg", os.O_WRONLY|os.O_CREATE, 0600)
	//		if errSImg := jpeg.Encode(f, img, nil); errSImg != nil {
	//			fmt.Println(errSImg)
	//		}
	//		f.Close()
	//		imgSlice = append(imgSlice, outImgName+strconv.Itoa(i)+".jpg")
	//	}
	//	if len(imgSlice) > 1 {
	//		for j := 1; j < count; j++ {
	//			MergeImageNew(outImgName+".jpg", outImgName+strconv.Itoa(j)+".jpg", outImgName)
	//		}
	//	}
	//	d.CloseDocument()
	//}
	//C.FPDF_DestroyLibrary()
	render.PdfToImg(os.Args[1],os.Args[2])
}

const MaxWidth float64 = 600

func fixSize(img1W, img2W int) (new1W, new2W int) {
	var ( //为了方便计算，将两个图片的宽转为 float64
		img1Width, img2Width = float64(img1W), float64(img2W)
		ratio1, ratio2       float64
	)

	minWidth := math.Min(img1Width, img2Width) // 取出两张图片中宽度最小的为基准

	if minWidth > 600 { // 如果最小宽度大于600，那么两张图片都需要进行缩放
		ratio1 = MaxWidth / img1Width // 图片1的缩放比例
		ratio2 = MaxWidth / img2Width // 图片2的缩放比例

		// 原宽度 * 比例 = 新宽度
		return int(img1Width * ratio1), int(img2Width * ratio2)
	}

	// 如果最小宽度小于600，那么需要将较大的图片缩放，使得两张图片的宽度一致
	if minWidth == img1Width {
		ratio2 = minWidth / img2Width // 图片2的缩放比例
		return img1W, int(img2Width * ratio2)
	}

	ratio1 = minWidth / img1Width // 图片1的缩放比例
	return int(img1Width * ratio1), img2W
}

func MergeImageNew(basePath string, maskPath string, outImageName string) {
	file1, _ := os.Open(basePath) //打开图片1
	file2, _ := os.Open(maskPath) //打开图片2
	defer file1.Close()
	defer file2.Close()

	// image.Decode 图片
	var (
		img1, img2 image.Image
		err        error
	)
	if img1, _, err = image.Decode(file1); err != nil {
		log.Fatal(err)
		return
	}
	if img2, _, err = image.Decode(file2); err != nil {
		log.Fatal(err)
		return
	}
	b1 := img1.Bounds()
	b2 := img2.Bounds()
	new1W, new2W := fixSize(b1.Max.X, b2.Max.X)

	// 调用resize库进行图片缩放(高度填0，resize.Resize函数中会自动计算缩放图片的宽高比)
	m1 := resize.Resize(uint(new1W), 0, img1, resize.Lanczos3)
	m2 := resize.Resize(uint(new2W), 0, img2, resize.Lanczos3)

	// 将两个图片合成一张
	newWidth := m1.Bounds().Max.X                                                                          //新宽度 = 随意一张图片的宽度
	newHeight := m1.Bounds().Max.Y + m2.Bounds().Max.Y                                                     // 新图片的高度为两张图片高度的和
	newImg := image.NewNRGBA(image.Rect(0, 0, newWidth, newHeight))                                        //创建一个新RGBA图像
	draw.Draw(newImg, newImg.Bounds(), m1, m1.Bounds().Min, draw.Over)                                     //画上第一张缩放后的图片
	draw.Draw(newImg, newImg.Bounds(), m2, m2.Bounds().Min.Sub(image.Pt(0, m1.Bounds().Max.Y)), draw.Over) //画上第二张缩放后的图片（这里需要注意Y值的起始位置）

	// 保存文件
	os.Remove(outImageName + ".jpg")
	imgFile, _ := os.Create(outImageName + ".jpg")
	defer imgFile.Close()
	jpeg.Encode(imgFile, newImg, &jpeg.Options{100})
}

