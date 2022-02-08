package render

// #cgo pkg-config: pdfium
// #include "fpdfview.h"
// #include "fpdf_annot.h"
// #include "fpdf_edit.h"
// #include "fpdf_structtree.h"
import "C"

import (
	"errors"
	"fmt"
	"github.com/nfnt/resize"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io/ioutil"
	"log"
	"math"
	"os"
	"strconv"
	"sync"
	"unsafe"
)

// Document is good
type Document struct {
	doc  C.FPDF_DOCUMENT
	data *[]byte // Keep a refrence to the data otherwise wierd stuff happens
}

const MaxWidth float64 = 600

var mutex = &sync.Mutex{}

// NewDocument shoud have docs
func NewDocument(data *[]byte) (*Document, error) {
	mutex.Lock()
	defer mutex.Unlock()
	// doc := C.FPDF_LoadDocument(C.CString("in.pdf"), nil)
	doc := C.FPDF_LoadMemDocument(
		unsafe.Pointer(&((*data)[0])),
		C.int(len(*data)),
		nil)

	if doc == nil {
		var errMsg string

		//defer C.FPDF_CloseDocument(doc)
		errorcase := C.FPDF_GetLastError()
		switch errorcase {
		case C.FPDF_ERR_SUCCESS:
			errMsg = "Success"
		case C.FPDF_ERR_UNKNOWN:
			errMsg = "Unknown error"
		case C.FPDF_ERR_FILE:
			errMsg = "Unable to read file"
		case C.FPDF_ERR_FORMAT:
			errMsg = "Incorrect format"
		case C.FPDF_ERR_PASSWORD:
			errMsg = "Invalid password"
		case C.FPDF_ERR_SECURITY:
			errMsg = "Invalid encryption"
		case C.FPDF_ERR_PAGE:
			errMsg = "Incorrect page"
		default:
			errMsg = "Unexpected error"
		}
		return nil, errors.New(errMsg)
	}
	return &Document{doc: doc, data: data}, nil
}

// GetPageCount shoud have docs
func (d *Document) GetPageCount() int {
	mutex.Lock()
	defer mutex.Unlock()
	return int(C.FPDF_GetPageCount(d.doc))
}

// CloseDocument shoud have docs
func (d *Document) Close() {
	mutex.Lock()
	C.FPDF_CloseDocument(d.doc)
	mutex.Unlock()
}

// RenderPage should have docs
func (d *Document) RenderPage(i int, dpi int) *image.RGBA {
	mutex.Lock()

	page := C.FPDF_LoadPage(d.doc, C.int(i))
	scale := float64(dpi) / 72.0
	imgWidth := C.FPDF_GetPageWidth(page) * C.double(scale)
	imgHeight := C.FPDF_GetPageHeight(page) * C.double(scale)

	// pixelBound := int(dpi * (3508 / 300))
	// imgWidthRatio := float64(pixelBound) / float64(imgWidth)
	// imgHeightRatio := float64(pixelBound) / float64(imgHeight)
	// scaleFactor := math.Min(imgWidthRatio, imgHeightRatio)
	scaleFactor := 1.0

	width := C.int(imgWidth * C.double(scaleFactor))
	height := C.int(imgHeight * C.double(scaleFactor))

	alpha := C.FPDFPage_HasTransparency(page)

	//创建空白位图对象
	bitmap := C.FPDFBitmap_Create(width, height, alpha)

	fillColor := 4294967295
	if int(alpha) == 1 {
		fillColor = 0
	}
	C.FPDFBitmap_FillRect(bitmap, 0, 0, width, height, C.ulong(fillColor))
	C.FPDF_RenderPageBitmap(bitmap, page, 0, 0, width, height, 0, C.FPDF_ANNOT) //C.FPDF_ANNOT 彩色|C.FPDF_GRAYSCALE 黑白

	p := C.FPDFBitmap_GetBuffer(bitmap)

	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	img.Stride = int(C.FPDFBitmap_GetStride(bitmap))
	mutex.Unlock()

	// This takes a bit of time and I *think* we can do this without the lock
	bgra := make([]byte, 4)
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			for i := range bgra {
				bgra[i] = *((*byte)(p))
				p = unsafe.Pointer(uintptr(p) + 1)
			}
			color := color.RGBA{B: bgra[0], G: bgra[1], R: bgra[2], A: bgra[3]}
			img.SetRGBA(x, y, color)
		}
	}
	mutex.Lock()
	C.FPDFBitmap_Destroy(bitmap)
	C.FPDF_ClosePage(page)
	mutex.Unlock()

	// should maybe return err
	//println(C.FPDF_GetLastError())

	return img
}

func InitLibrary() {
	mutex.Lock()
	C.FPDF_InitLibrary()
	mutex.Unlock()
}

func DestroyLibrary() {
	mutex.Lock()
	C.FPDF_DestroyLibrary()
	mutex.Unlock()
}

// FixSize 图片拼接之前计算  宽度尺寸
func FixSize(img1W, img2W int) (new1W, new2W int) {
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

// MergeImageNew  拼接图片
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
	new1W, new2W := FixSize(b1.Max.X, b2.Max.X)

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

// PdfToImg PDF 转 图片 并拼接后保存
func PdfToImg(filePath string,outName string){
	data, _ := ioutil.ReadFile(filePath)
	outImgName := "./outImg/"+outName
	//C.FPDF_InitLibrary()
	InitLibrary()
	d, err := NewDocument(&data)
	if err != nil {
		println(err)
	} else {
		count := d.GetPageCount()
		img0 := d.RenderPage(0, 600)
		fb, _ := os.OpenFile(outImgName+".jpg", os.O_WRONLY|os.O_CREATE, 0600)
		jpeg.Encode(fb, img0, nil)
		fb.Close()
		imgSlice := make([]string, 0, 30)
		for i := 0; i < count; i++ {

			img := d.RenderPage(i, 600)
			f, _ := os.OpenFile(outImgName+strconv.Itoa(i)+".jpg", os.O_WRONLY|os.O_CREATE, 0600)
			if errSImg := jpeg.Encode(f, img, nil); errSImg != nil {
				fmt.Println(errSImg)
			}
			f.Close()
			imgSlice = append(imgSlice, outImgName+strconv.Itoa(i)+".jpg")
		}
		if len(imgSlice) > 1 {
			for j := 1; j < count; j++ {
				MergeImageNew(outImgName+".jpg", outImgName+strconv.Itoa(j)+".jpg", outImgName)
				os.Remove(outImgName+strconv.Itoa(j)+".jpg") //删除单图
			}
		}
		os.Remove(outImgName+strconv.Itoa(0)+".jpg")
		d.Close()
	}
	DestroyLibrary()
}
