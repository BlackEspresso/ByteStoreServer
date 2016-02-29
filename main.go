package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"./bytestore"

	"github.com/gin-gonic/gin"
	"github.com/satori/go.uuid"
)

var manager *bytestore.ContainerManager = bytestore.NewContainerManager()
var tokens map[string]*DownloadToken = map[string]*DownloadToken{}

type DownloadToken struct {
	FileId      string
	ContainerId string
	Token       string
}

func main() {
	bytestore.CheckWorkingDirExists()
	manager.ReadFromDir()

	r := gin.Default()
	r.POST("/file/:container", upload)

	r.GET("/file/:container/:file", download)
	r.GET("/info", infoContainers)
	r.POST("/token/:container/:file", createDownloadToken)
	r.GET("/info/:container", infoContainer)
	r.GET("/info/:container/:file", infoFile)

	r.DELETE("/file/:container/:file", deleteFile)
	r.DELETE("/file/:container", deleteContainer)

	r.Static("/static/", "./static/")

	go func() {
		r.Run("localhost:8079")
	}()

	rFrontEnd := gin.Default()
	rFrontEnd.GET("/downloadbytoken/:token", downloadByToken)
	rFrontEnd.Run(":8080")
}

func downloadByToken(g *gin.Context) {
	token := g.Param("token")
	td, exists := tokens[token]
	if !exists {
		g.String(404, "download not found or token expired")
	}
	cId, _ := uuid.FromString(td.ContainerId)
	fId, _ := uuid.FromString(td.FileId)
	container, ok := manager.GetContainer(cId)
	if !ok {
		g.String(404, "container not found")
		return
	}
	fm, ok := container.GetFile(fId)
	if !ok {
		g.String(404, "file not found")
		return
	}

	filePath := container.GetFilePath(fm.Id)
	downloadFileFromPath(g, filePath, fm.FileName)
	delete(tokens, token)
}

func createDownloadToken(g *gin.Context) {
	c, fm, ok := getFileFromRequest(g)
	if !ok {
		return
	}

	token := newToken()
	dt := DownloadToken{}
	dt.ContainerId = c.Id.String()
	dt.FileId = fm.Id.String()
	dt.Token = token

	tokens[dt.Token] = &dt

	g.JSON(200, dt)
}

func newToken() string {
	u1 := uuid.NewV4()
	u2 := uuid.NewV4()
	token := u1.String() + u2.String()
	token = strings.Replace(token, "-", "", -1)
	return token
}

func upload(g *gin.Context) {
	cId, ok := getContainerId(g)
	if !ok {
		return
	}

	c := manager.GetOrCreateContainer(cId)

	file, header, err := g.Request.FormFile("upload")
	if err != nil {
		g.String(404, err.Error())
		return
	}
	filename := header.Filename
	meta := g.Request.FormValue("meta")
	fm := c.AddFile(filename, meta, file)
	g.JSON(200, fm)
}

func download(g *gin.Context) {
	container, fm, ok := getFileFromRequest(g)
	if !ok {
		return
	}
	filePath := container.GetFilePath(fm.Id)
	downloadFileFromPath(g, filePath, fm.FileName)
}

func downloadFileFromPath(g *gin.Context, filePath, fileName string) {
	st, err := os.Stat(filePath)
	if err != nil {
		fmt.Print(err)
		return
	}
	g.Header("Content-Disposition", "attachment; filename='"+fileName+"';")
	g.Header("Content-Type", "application/octet-stream")
	g.Header("Content-Length", strconv.Itoa(int(st.Size())))
	f, err := os.Open(filePath)
	if err != nil {
		log.Println(err)
		g.String(500, "cant stream file")
		return
	}
	defer f.Close()
	io.Copy(g.Writer, f)
}

func deleteContainer(g *gin.Context) {
	container, ok := getContainerFromRequest(g)
	if !ok {
		return
	}
	container.Delete()
}

func deleteFile(g *gin.Context) {
	container, fm, ok := getFileFromRequest(g)
	if !ok {
		return
	}
	container.DeleteFile(fm)
}

func infoContainers(g *gin.Context) {
	l := manager.GetContainers(300)
	g.JSON(200, l)
}

func infoContainer(g *gin.Context) {
	c, ok := getContainerFromRequest(g)
	if !ok {
		return
	}
	l := c.GetFiles(300)
	g.JSON(200, l)
}

func infoFile(g *gin.Context) {
	_, fm, ok := getFileFromRequest(g)
	if !ok {
		return
	}

	g.JSON(200, fm)
}

func getContainerId(g *gin.Context) (uuid.UUID, bool) {
	cIdStr := g.Param("container")
	cId, err1 := uuid.FromString(cIdStr)
	if err1 != nil {
		g.String(404, "container id invalid")
		return uuid.Nil, false
	}
	return cId, true
}

func getFileId(g *gin.Context) (uuid.UUID, bool) {
	fIdStr := g.Param("file")
	fId, err1 := uuid.FromString(fIdStr)
	if err1 != nil {
		g.String(404, "file id invalid")
		return uuid.Nil, false
	}
	return fId, true
}

func getContainerFromRequest(g *gin.Context) (*bytestore.Container, bool) {
	cId, ok := getContainerId(g)
	if !ok {
		return nil, false
	}
	container, ok := manager.GetContainer(cId)
	if !ok {
		g.String(404, "container not found")
		return nil, false
	}
	return container, true
}

func getFileFromRequest(g *gin.Context) (*bytestore.Container, *bytestore.FileMeta, bool) {
	container, ok := getContainerFromRequest(g)
	if !ok {
		return nil, nil, false
	}
	fId, ok := getFileId(g)
	if !ok {
		return nil, nil, false
	}

	fm, ok := container.GetFile(fId)
	if !ok {
		g.String(404, "fileid not found")
		return nil, nil, false
	}
	return container, fm, true
}
