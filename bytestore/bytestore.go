package bytestore

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/satori/go.uuid"
)

type ContainerList map[uuid.UUID]*Container
type FileList map[uuid.UUID]*FileMeta

var workingDir = "./containers"
var errorFileNotInContainer error = errors.New("file not in container")

type ContainerManager struct {
	cLock      sync.Locker
	containers ContainerList
}

type Container struct {
	Id    uuid.UUID
	files FileList
	fLock sync.Locker
}

type FileMeta struct {
	Id          uuid.UUID
	FileName    string
	ContainerId uuid.UUID
	Meta        string
	CreatedDate time.Time
}

func NewContainerManager() *ContainerManager {
	cs := new(ContainerManager)
	cs.containers = ContainerList{}
	cs.cLock = &sync.Mutex{}
	return cs
}

func newContainer() *Container {
	c := Container{}
	c.Id = uuid.NewV4()
	c.files = FileList{}
	c.fLock = &sync.Mutex{}

	return &c
}

func (m *ContainerManager) addToList(c *Container) {
	m.cLock.Lock()
	m.containers[c.Id] = c
	m.cLock.Unlock()
}

func (m *ContainerManager) GetContainer(id uuid.UUID) (*Container, bool) {
	c, ok := m.containers[id]
	return c, ok
}

func (m *ContainerManager) GetContainers(ipp int) []string {
	list := []string{}
	for m := range m.containers {
		if len(list) < ipp {
			list = append(list, m.String())
		}
	}
	return list
}

func (m *Container) GetFiles(ipp int) []string {
	list := []string{}
	for m := range m.files {
		if len(list) < ipp {
			list = append(list, m.String())
		}
	}
	return list
}

func (c *Container) GetFile(id uuid.UUID) (*FileMeta, bool) {
	fm, ok := c.files[id]
	return fm, ok
}

func (m *ContainerManager) GetOrCreateContainer(id uuid.UUID) *Container {
	c, ok := m.GetContainer(id)
	if ok {
		return c
	}

	c = newContainer()
	c.Id = id
	os.Mkdir(c.GetPath(), 0777)
	m.addToList(c)
	return c
}

func (c *Container) GetFilePath(fileId uuid.UUID) string {
	return path.Join(workingDir, c.Id.String(), fileId.String()+".bin")
}
func (c *Container) GetMetaFilePath(fileId uuid.UUID) string {
	return path.Join(workingDir, c.Id.String(), fileId.String()+".json")
}

func (container *Container) GetPath() string {
	return path.Join(workingDir, container.Id.String())
}

func (container *Container) Delete() error {
	path := container.GetPath()
	return os.RemoveAll(path)
}

func (container *Container) DeleteFile(fm *FileMeta) error {
	if fm.ContainerId == container.Id {
		fp := container.GetFilePath(fm.Id)
		fmp := container.GetMetaFilePath(fm.Id)
		err1 := os.Remove(fp)
		err2 := os.Remove(fmp)
		if err1 != nil {
			return err1
		}
		if err2 != nil {
			return err2
		}
	}
	return errorFileNotInContainer
}

func (container *Container) AddFile(name string, meta string, reader io.Reader) *FileMeta {
	f := FileMeta{}
	f.Id = uuid.NewV4()
	f.ContainerId = container.Id
	f.FileName = name
	f.CreatedDate = time.Now()
	f.Meta = meta

	// write file content
	{
		out, err := os.Create(container.GetFilePath(f.Id))
		if err != nil {
			log.Fatal(err)
		}
		defer out.Close()
		_, err = io.Copy(out, reader)
		if err != nil {
			log.Fatal(err)
		}
	}

	metaFileContent, err := json.Marshal(f)
	if err != nil {
		log.Fatal(err)
	}
	// write meta content
	ioutil.WriteFile(container.GetMetaFilePath(f.Id), metaFileContent, 0777)

	// register file in index
	container.fLock.Lock()
	container.files[f.Id] = &f
	container.fLock.Unlock()
	return &f
}

func (container *Container) ReadFromDir() {
	files, err := ioutil.ReadDir(container.GetPath())
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		uid, err := uuid.FromString(file.Name())
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		if err != nil {
			log.Println(err)
			continue
		}
		fm := new(FileMeta)
		metaContent, err := ioutil.ReadFile(container.GetMetaFilePath(uid))
		if err != nil {
			log.Fatal(err)
		}
		err = json.Unmarshal(metaContent, &fm)
		if err != nil {
			log.Fatal(err)
		}

		container.files[uid] = fm
	}
}

func CheckWorkingDirExists() {
	if ok, _ := exists(workingDir); !ok {
		os.Mkdir(workingDir, 0777)
	}
}

func (cm *ContainerManager) ReadFromDir() {
	folders, err := ioutil.ReadDir(workingDir)
	if err != nil {
		log.Fatal(err)
	}

	//read folders
	for _, folder := range folders {
		containerId, err := uuid.FromString(folder.Name())
		if err != nil {
			log.Println(err)
			continue
		}
		c := newContainer()
		c.Id = containerId
		cm.addToList(c)
		c.ReadFromDir()
	}
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}
