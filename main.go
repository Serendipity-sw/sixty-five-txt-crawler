package main

import (
	"bytes"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/guotie/config"
	"github.com/smtc/glog"
	"github.com/swgloomy/gutil"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	titleDom   string
	contentDom string
)

func main() {
	defer func() {
		glog.Close()
		os.Exit(0)
	}()
	gutil.LogInit(true, "./logs")

	runtime.GOMAXPROCS(runtime.NumCPU())
	glog.Info("set many cpu successfully \n")

	err := config.ReadCfg("./config.json")
	if err != nil {
		glog.Error("configProcess read config err! err: %s \n", err.Error())
		return
	}

	urlPath := config.GetString("url")

	urlObj, err := url.Parse(urlPath)
	if err != nil {
		glog.Error("url parse run err! urlPath: %s err: %s \n", urlPath, err.Error())
		return
	}

	threadSleepTime := config.GetIntDefault("threadSleepTime", 0)
	threadSyncNumber := config.GetIntDefault("threadSyncNumber", 0)

	catalogDom := config.GetString("catalogDom")
	titleDom = config.GetString("titleDom")
	contentDom = config.GetString("contentDom")

	pageListDomIn, err := getUrlDom(urlPath)
	if err != nil {
		glog.Error("getUrlDom run err! urlPath: %s err: %s \n", urlPath, err.Error())
		return
	}

	docQuery, err := goquery.NewDocumentFromReader(bytes.NewBuffer(*pageListDomIn))
	if err != nil {
		glog.Error("NewDocumentFromReader run err! pageListDomIn: %s err: %s \n", string(*pageListDomIn), err.Error())
		return
	}

	var (
		bo      bool
		urlList []string
	)

	dtNumber := docQuery.Find(fmt.Sprintf("%s dl>dt", catalogDom)).Length()

	docQuery.Find(fmt.Sprintf("%s dl>*", catalogDom)).Each(func(i int, selection *goquery.Selection) {
		if dtNumber > 0 {
			if selection.Is("dt") {
				dtNumber--
			}
		} else {
			urlPath, bo = selection.Find("a").Attr("href")
			if bo {
				if strings.HasPrefix(urlPath, "/") {
					urlPath = fmt.Sprintf("%s://%s/%s", urlObj.Scheme, urlObj.Host, urlPath)
				}
				urlList = append(urlList, urlPath)
			}
		}
	})

	var (
		threadLock      sync.WaitGroup
		contentList     = make(map[int]string)
		contentListLock sync.RWMutex
		contentArray    []byte
	)
	contentArray = append(contentArray, []byte(fmt.Sprintf("%s\r\n", urlPath))...)
	for i, s := range urlList {
		contentList[i] = ""
		if threadSyncNumber != 0 {
			if i%threadSyncNumber == 0 {
				threadLock.Wait()
			}
		} else {
			threadLock.Wait()
		}
		threadLock.Add(1)
		go fictionPageProcess(s, &contentListLock, &threadLock, &contentList, i)
		if threadSleepTime != 0 {
			time.Sleep(time.Duration(threadSleepTime) * time.Second)
		}
	}
	threadLock.Wait()

	var keys []int
	for k := range contentList {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	for _, s := range keys {
		contentArray = append(contentArray, []byte(contentList[s])...)
		contentArray = append(contentArray, []byte("\r\n")...)
	}
	fileName := "小说.txt"
	err = gutil.FileCreateAndWrite(&contentArray, fileName, false)
	if err != nil {
		glog.Error("FileCreateAndWrite run err! content: %s fileName: %s err: %s \n", string(contentArray), fileName, err.Error())
		return
	}
	glog.Info("main run success! fileName: %s \n", fileName)
}

func getUrlDom(urlPath string) (*[]byte, error) {
	result, err := http.Get(urlPath)
	if err != nil {
		glog.Error("getUrlDom get run err! urlPath: %s err: %s \n", urlPath, err.Error())
		return nil, err
	}
	defer func() {
		err = result.Body.Close()
		if err != nil {
			glog.Error("getUrlDom body close err! urlPath: %s err: %s \n", urlPath, err.Error())
		}
	}()
	domByte, err := ioutil.ReadAll(result.Body)
	if err != nil {
		glog.Error("getUrlDom ReadAll body err! urlPath: %s err: %s \n", urlPath, err.Error())
		return nil, err
	}

	return &domByte, nil
}

/**
小说页面处理
*/
func fictionPageProcess(httpUrl string, contentListLock *sync.RWMutex, threadLock *sync.WaitGroup, contentListIn *map[int]string, index int) {
	defer func() {
		threadLock.Done()
	}()
	result, err := http.Get(httpUrl)
	if err != nil {
		glog.Error("fictionPageProcess get err! httpUrl: %s index: %d err: %s \n", httpUrl, index, err.Error())
		return
	}
	defer func() {
		err = result.Body.Close()
		if err != nil {
			glog.Error("fictionPageProcess body close err! httpUrl: %s index: %d err: %s \n", httpUrl, index, err.Error())
		}
	}()
	docQuery, err := goquery.NewDocumentFromReader(result.Body)
	if err != nil {
		glog.Error("fictionPageProcess NewDocumentFromReader run err! httpUrl: %s index: %d err: %s \n", httpUrl, index, err.Error())
		return
	}
	var content string
	content = docQuery.Find(titleDom).Eq(0).Text()
	content = fmt.Sprintf("%s\r\n%s", content, docQuery.Find(contentDom).Text())
	contentListLock.Lock()
	(*contentListIn)[index] = strings.ReplaceAll(content, "    ", "\r\n")
	contentListLock.Unlock()
	glog.Info("fictionPageProcess run success! httpUrl: %s index: %d \n", httpUrl, index)
}
