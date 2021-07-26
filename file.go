package logfile

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	defaultMaxFileSize int64       = 10 << 20                 // 10m
	defaultPath                    = "./log"                  // 默认日志路径
	defaultFileName                = "file.log"               // 默认日志文件名
	defaultFormat                  = "%Y%m%d"                 // 默认文件后缀
	defaultMaxAge                  = 7 * 24 * time.Hour       // 文件最大留存时间(7天)
	defaultPerm        os.FileMode = 0644                     // 文件权限
	PathSeparator                  = string(os.PathSeparator) // 目录分隔符
	defaultTicker                  = 1                        // 清理过期文件,默认每天清理一次
)

const (
	year   = "2006"
	month  = "01"
	day    = "02"
	hour   = "15"
	minute = "04"
	second = "05"
)

type Config struct {
	Level       uint8         // 等级
	Path        string        // 路径
	FileName    string        // 文件名
	Format      string        // 文件匹配格式 %Y%m%d
	MaxFileSize int64         // 文件最大size
	IsLevelFile bool          // 是否分level输出日志，默认不开启
	maxAge      time.Duration // 文件最大留存时间
	isAP        bool          // 是否是绝对路径
}

type FileHook struct {
	*Config
	mutex        sync.RWMutex // 读写锁
	currentSize  int64        // 当前文件size
	currentIndex int          // 当前文件index
	currentFile  os.FileInfo  // 当前文件信息
	file         *os.File     // 当前文件
}

func NewFileHook(c *Config) (h *FileHook, err error) {
	if c.Path == "" {
		c.Path = defaultPath
	}
	if c.FileName == "" {
		c.FileName = defaultFileName
	}
	if c.Format == "" {
		c.Format = defaultFormat
	}
	if c.MaxFileSize <= 0 {
		c.MaxFileSize = defaultMaxFileSize
	}
	if c.maxAge <= 0 {
		c.maxAge = defaultMaxAge
	}
	if err = c.dealPath(); err != nil {
		return
	}
	c.isAP = path.IsAbs(c.Path)
	h = &FileHook{
		Config: c,
	}
	h.dealLogFile()
	err = h.openFile()
	go func() {
		ticker := time.NewTicker(time.Minute * time.Duration(defaultTicker))
		for _ = range ticker.C {
			h.removeFiles() // 移除过期文件
		}
	}()
	return
}

func (c *Config) dealPath() (err error) {
	s, err := os.Stat(c.Path)
	if err != nil {
		return os.MkdirAll(c.Path, os.ModePerm)
	}
	if !s.IsDir() {
		return errors.New("path is a file")
	}
	return
}

// 打开最新文件
func (h *FileHook) openFile() (err error) {
	h.file, err = os.OpenFile(h.getCurrentFileName(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, defaultPerm)
	if err != nil {
		return
	}
	h.linkFile()
	return
}

func (h *FileHook) dealFormat() {
	h.Format = strings.ReplaceAll(h.Format, "%Y", year)
	h.Format = strings.ReplaceAll(h.Format, "%m", month)
	h.Format = strings.ReplaceAll(h.Format, "%d", day)
	h.Format = strings.ReplaceAll(h.Format, "%H", hour)
	h.Format = strings.ReplaceAll(h.Format, "%M", minute)
	h.Format = strings.ReplaceAll(h.Format, "%S", second)
}
func (h *FileHook) removeFiles() {
	matchStr := ""
	timeStr := time.Now().Format(h.Format)
	if timeStr == "" {
		matchStr = fmt.Sprintf("%s.[0-9]?", h.FileName)
	} else {
		matchStr = fmt.Sprintf("%s.%s.[0-9]?", h.FileName, timeStr)
	}
	files, _ := ioutil.ReadDir(h.Path)
	for _, f := range files {
		// 筛选所有filename 开头的 filename*
		if matched, _ := regexp.MatchString(matchStr, f.Name()); matched {
			if f.IsDir() {
				fmt.Println("目录:", f.Name())
				continue
			}
			if !f.ModTime().Add(h.maxAge).After(time.Now()) {
				//最后修改日期 +  最大保留日期  是否超过当前时间超过则删除
				_ = os.Remove(path.Join(h.Path, f.Name()))
				fmt.Println("删除文件:", f.Name())
				continue
			}
		}
	}
}

func (h *FileHook) dealFiles() {
	matchStr := ""
	timeStr := time.Now().Format(h.Format)
	if timeStr == "" {
		matchStr = fmt.Sprintf("%s.[0-9]?", h.FileName)
	} else {
		matchStr = fmt.Sprintf("%s.%s.[0-9]?", h.FileName, timeStr)
	}
	files, _ := ioutil.ReadDir(h.Path)
	for _, f := range files {
		// 筛选所有filename 开头的 filename*
		fmt.Println(regexp.MatchString(matchStr, f.Name()))
		if matched, _ := regexp.MatchString(matchStr, f.Name()); matched {
			if f.IsDir() {
				fmt.Println("目录:", f.Name())
				continue
			}
			names := strings.Split(f.Name(), ".")
			index, err := strconv.Atoi(names[len(names)-1])
			if err != nil {
				fmt.Println("错误index", names[len(names)-1])
			}
			if index > h.currentIndex {
				h.currentIndex = index
				h.currentFile = f
			}
		}
	}
	// 判断当前文件是否超过最大文件size;如果超过则重新创建文件
	if h.currentFile == nil || h.currentFile.Size() >= h.MaxFileSize {
		h.currentIndex += 1
		h.currentSize = 0
	} else {
		h.currentSize = h.currentFile.Size()
	}
}
func (h *FileHook) isNotExist(filename string) bool {
	_, err := os.Stat(filename)
	return err != nil
}

func (h *FileHook) linkFile() {
	cmd := exec.Command("ln", "-snf", h.getNoPathFileName(), h.getLinkName())
	err := cmd.Run()
	fmt.Println(err)
}
func (h *FileHook) getNoPathFileName() string {
	timeStr := time.Now().Format(h.Format)
	if timeStr == "" {
		return fmt.Sprintf("%s%v", h.FileName, h.currentIndex)
	}
	return fmt.Sprintf("%s.%s.%v", h.FileName, timeStr, h.currentIndex)
}

func (h *FileHook) getCurrentFileName() string {
	timeStr := time.Now().Format(h.Format)
	if timeStr == "" {
		return fmt.Sprintf("%s%s%s%v", h.Path, PathSeparator, h.FileName, h.currentIndex)
	}
	return fmt.Sprintf("%s%s%s.%s.%v", h.Path, PathSeparator, h.FileName, timeStr, h.currentIndex)
}
func (h *FileHook) getLinkName() string {
	return fmt.Sprintf("%s%s%v", h.Path, PathSeparator, h.FileName)
}
func (h *FileHook) dealLogFile() {
	h.dealFormat() // 处理匹配
	h.dealFiles()  // 处理文件及获取当前index以及获取当前日志文件size
}
func (h *FileHook) Write(line []byte) (err error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if h.currentSize+int64(len(line)) >= h.MaxFileSize {
		h.currentSize = 0
		err = h.file.Close()
		h.currentIndex += 1
		_ = h.openFile()
	}
	var size int
	size, err = h.file.Write(line)
	if err != nil {
		return
	}
	h.currentSize += int64(size)
	return
}
