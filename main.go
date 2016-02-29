package main

import (
	"fmt"

	"./bytestore"

	"github.com/gin-gonic/gin"
	"github.com/satori/go.uuid"
)

var manager *bytestore.ContainerManager = bytestore.NewContainerManager()

func main() {
	bytestore.CheckWorkingDirExists()
	manager.ReadFromDir()

	r := gin.Default()
	r.POST("/file/:container", upload)

	r.GET("/file/:container/:file", download)
	r.GET("/info", infoContainers)
	r.GET("/info/:container", infoContainer)
	r.GET("/info/:container/:file", infoFile)

	r.DELETE("/file/:container/:file", deleteFile)
	r.DELETE("/file/:container", deleteContainer)

	r.Static("/static/", "./static/")

	r.Run(":8080")
}

func upload(g *gin.Context) {
	cIdStr := g.Param("container")
	cId, err := uuid.FromString(cIdStr)
	if err != nil {
		g.String(404, "container id invalid")
		return
	}

	c := manager.GetOrCreateContainer(cId)

	file, header, err := g.Request.FormFile("upload")
	if err != nil {
		g.String(404, err.Error())
		return
	}
	filename := header.Filename
	fmt.Println(header.Filename)
	meta := g.Request.FormValue("meta")
	fmt.Println(meta)
	fm := c.AddFile(filename, meta, file)
	g.JSON(200, fm)
}

func download(g *gin.Context) {
	container, fm, ok := getFileFromRequest(g)
	if !ok {
		return
	}
	filePath := container.GetFilePath(fm.Id)
	g.Header("Content-Disposition", "attachment; filename='"+fm.FileName+"';")
	g.File(filePath)
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

func getContainerFromRequest(g *gin.Context) (*bytestore.Container, bool) {
	cIdStr := g.Param("container")
	cId, err1 := uuid.FromString(cIdStr)
	if err1 != nil {
		g.String(404, "container id invalid")
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
	fIdStr := g.Param("file")
	fId, err := uuid.FromString(fIdStr)

	if err != nil {
		g.String(404, "file id invalid")
		return nil, nil, false
	}

	fm, ok := container.GetFile(fId)
	if !ok {
		g.String(404, "fileid not found")
		return nil, nil, false
	}
	return container, fm, true
}
