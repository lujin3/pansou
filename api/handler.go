package api

import (
	// "fmt"
	"net/http"
	"os"

	"io"
	"mime"
	"net/url"
	"pansou/config"
	"pansou/model"
	"pansou/service"
	"pansou/util"
	jsonutil "pansou/util/json"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// 保存搜索服务的实例
var searchService *service.SearchService

// SetSearchService 设置搜索服务实例
func SetSearchService(service *service.SearchService) {
	searchService = service
}

// SearchHandler 搜索处理函数
func SearchHandler(c *gin.Context) {
	var req model.SearchRequest
	var err error

	// 根据请求方法不同处理参数
	if c.Request.Method == http.MethodGet {
		// GET方式：从URL参数获取
		// 获取keyword，必填参数
		keyword := c.Query("kw")
		
		// 处理channels参数，支持逗号分隔
		channelsStr := c.Query("channels")
		var channels []string
		// 只有当参数非空时才处理
		if channelsStr != "" && channelsStr != " " {
			parts := strings.Split(channelsStr, ",")
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					channels = append(channels, trimmed)
				}
			}
		}
		
		// 处理并发数
		concurrency := 0
		concStr := c.Query("conc")
		if concStr != "" && concStr != " " {
			concurrency = util.StringToInt(concStr)
		}
		
		// 处理强制刷新
		forceRefresh := false
		refreshStr := c.Query("refresh")
		if refreshStr != "" && refreshStr != " " && refreshStr == "true" {
			forceRefresh = true
		}
		
		// 处理结果类型和来源类型
		resultType := c.Query("res")
		if resultType == "" || resultType == " " {
			resultType = "merge" // 直接设置为默认值merge
		}
		
		sourceType := c.Query("src")
		if sourceType == "" || sourceType == " " {
			sourceType = "all" // 直接设置为默认值all
		}
		
		// 处理plugins参数，支持逗号分隔
		var plugins []string
		// 检查请求中是否存在plugins参数
		if c.Request.URL.Query().Has("plugins") {
			pluginsStr := c.Query("plugins")
			// 判断参数是否非空
			if pluginsStr != "" && pluginsStr != " " {
				parts := strings.Split(pluginsStr, ",")
				for _, part := range parts {
					trimmed := strings.TrimSpace(part)
					if trimmed != "" {
						plugins = append(plugins, trimmed)
					}
				}
			}
		} else {
			// 如果请求中不存在plugins参数，设置为nil
			plugins = nil
		}
		
		// 处理cloud_types参数，支持逗号分隔
		var cloudTypes []string
		// 检查请求中是否存在cloud_types参数
		if c.Request.URL.Query().Has("cloud_types") {
			cloudTypesStr := c.Query("cloud_types")
			// 判断参数是否非空
			if cloudTypesStr != "" && cloudTypesStr != " " {
				parts := strings.Split(cloudTypesStr, ",")
				for _, part := range parts {
					trimmed := strings.TrimSpace(part)
					if trimmed != "" {
						cloudTypes = append(cloudTypes, trimmed)
					}
				}
			}
		} else {
			// 如果请求中不存在cloud_types参数，设置为nil
			cloudTypes = nil
		}
		
		// 处理ext参数，JSON格式
		var ext map[string]interface{}
		extStr := c.Query("ext")
		if extStr != "" && extStr != " " {
			// 处理特殊情况：ext={}
			if extStr == "{}" {
				ext = make(map[string]interface{})
			} else {
				if err := jsonutil.Unmarshal([]byte(extStr), &ext); err != nil {
					c.JSON(http.StatusBadRequest, model.NewErrorResponse(400, "无效的ext参数格式: "+err.Error()))
					return
				}
			}
		}
		// 确保ext不为nil
		if ext == nil {
			ext = make(map[string]interface{})
		}

		req = model.SearchRequest{
			Keyword:      keyword,
			Channels:     channels,
			Concurrency:  concurrency,
			ForceRefresh: forceRefresh,
			ResultType:   resultType,
			SourceType:   sourceType,
			Plugins:      plugins,
			CloudTypes:   cloudTypes, // 添加cloud_types到请求中
			Ext:          ext,
		}
	} else {
		// POST方式：从请求体获取
		data, err := c.GetRawData()
		if err != nil {
			c.JSON(http.StatusBadRequest, model.NewErrorResponse(400, "读取请求数据失败: "+err.Error()))
			return
		}

		if err := jsonutil.Unmarshal(data, &req); err != nil {
			c.JSON(http.StatusBadRequest, model.NewErrorResponse(400, "无效的请求参数: "+err.Error()))
			return
		}
	}
	
	// 检查并设置默认值
	if len(req.Channels) == 0 {
		req.Channels = config.AppConfig.DefaultChannels
	}
	
	// 如果未指定结果类型，默认返回merge并转换为merged_by_type
	if req.ResultType == "" {
		req.ResultType = "merged_by_type"
	} else if req.ResultType == "merge" {
		// 将merge转换为merged_by_type，以兼容内部处理
		req.ResultType = "merged_by_type"
	}
	
	// 如果未指定数据来源类型，默认为全部
	if req.SourceType == "" {
		req.SourceType = "all"
	}
	
	// 参数互斥逻辑：当src=tg时忽略plugins参数，当src=plugin时忽略channels参数
	if req.SourceType == "tg" {
		req.Plugins = nil // 忽略plugins参数
	} else if req.SourceType == "plugin" {
		req.Channels = nil // 忽略channels参数
	} else if req.SourceType == "all" {
		// 对于all类型，如果plugins为空或不存在，统一设为nil
		if req.Plugins == nil || len(req.Plugins) == 0 {
			req.Plugins = nil
		}
	}
	
	// 可选：启用调试输出（生产环境建议注释掉）
	// fmt.Printf("🔧 [调试] 搜索参数: keyword=%s, channels=%v, concurrency=%d, refresh=%v, resultType=%s, sourceType=%s, plugins=%v, cloudTypes=%v, ext=%v\n", 
	//	req.Keyword, req.Channels, req.Concurrency, req.ForceRefresh, req.ResultType, req.SourceType, req.Plugins, req.CloudTypes, req.Ext)
	
	// 执行搜索
	result, err := searchService.Search(req.Keyword, req.Channels, req.Concurrency, req.ForceRefresh, req.ResultType, req.SourceType, req.Plugins, req.CloudTypes, req.Ext)
	
	if err != nil {
		response := model.NewErrorResponse(500, "搜索失败: "+err.Error())
		jsonData, _ := jsonutil.Marshal(response)
		c.Data(http.StatusInternalServerError, "application/json", jsonData)
		return
	}

	// 返回结果
	response := model.NewSuccessResponse(result)
	jsonData, _ := jsonutil.Marshal(response)
	c.Data(http.StatusOK, "application/json", jsonData)
} 


func PansouPage(c *gin.Context) {
    // 回退到前端自行校验token（localStorage），这里直接渲染页面
    c.HTML(http.StatusOK, "pansou.html", nil)
}

func DoubanPage(c *gin.Context) {
    // 回退到前端自行校验token（localStorage），这里直接渲染页面
    c.HTML(http.StatusOK, "douban.html", nil)
}

func TokenPage(c *gin.Context) { c.HTML(http.StatusOK, "token.html", nil) }

// VerifyTokenHandler 仅用于校验前端提交的 token（不设置任何 Cookie）
func VerifyTokenHandler(c *gin.Context) {
    var req struct{ Token string `json:"token" form:"token"` }
    _ = c.ShouldBind(&req)
    if req.Token == "" { req.Token = c.Query("token") }
    t := strings.TrimSpace(req.Token)
    if strings.HasPrefix(strings.ToLower(t), "bearer ") { t = strings.TrimSpace(t[7:]) }
    expected := os.Getenv("TOKEN")
    if expected == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "server token not configured"})
        return
    }
    if t == expected {
        c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
        return
    }
    c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "invalid token"})
}

// DoubanProxyHandler 代理请求到豆瓣 j/search_subjects，避免前端CORS问题
func DoubanProxyHandler(c *gin.Context) {
    // 支持两种用法：
    // 1) 直接透传 type/tag/page_limit/page_start/search_text
    // 2) 通过 cat=hot|movie|tv|variety 使用默认映射

    // 如果是 suggest 模式，转发到 subject_suggest
    if c.Query("suggest") == "1" || c.Query("endpoint") == "suggest" {
        qv := url.Values{}
        qParam := c.Query("q")
        if qParam != "" { qv.Set("q", qParam) }
        target := "https://movie.douban.com/j/subject_suggest?" + qv.Encode()

        client := util.GetHTTPClient()
        req, err := http.NewRequest("GET", target, nil)
        if err != nil {
            c.JSON(http.StatusInternalServerError, model.NewErrorResponse(500, "创建请求失败: "+err.Error()))
            return
        }
        req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0 Safari/537.36")
        req.Header.Set("Accept", "application/json, text/plain, */*")
        req.Header.Set("Referer", "https://movie.douban.com/")

        resp, err := client.Do(req)
        if err != nil {
            c.JSON(http.StatusBadGateway, model.NewErrorResponse(502, "请求豆瓣失败: "+err.Error()))
            return
        }
        defer resp.Body.Close()

        body, err := io.ReadAll(resp.Body)
        if err != nil {
            c.JSON(http.StatusBadGateway, model.NewErrorResponse(502, "读取豆瓣响应失败: "+err.Error()))
            return
        }
        if resp.StatusCode != 200 {
            c.JSON(http.StatusBadGateway, model.NewErrorResponse(502, "豆瓣响应状态异常: "+resp.Status))
            return
        }
        c.Data(http.StatusOK, "application/json; charset=utf-8", body)
        return
    }

    // 默认参数（search_subjects）
    q := url.Values{}
    pageLimit := c.DefaultQuery("page_limit", "20")
    pageStart := c.DefaultQuery("page_start", "0")
    searchText := c.Query("search_text")
    typ := c.Query("type")
    tag := c.Query("tag")
    cat := c.Query("cat")

    // 类别映射（仅在未显式传入type/tag时生效）
    if typ == "" && tag == "" {
        switch cat {
        case "hot":
            typ = "movie"
            tag = "热门"
        case "movie":
            typ = "movie"
            tag = "热门"
        case "tv":
            typ = "tv"
            tag = "热门"
        case "variety":
            typ = "tv"
            tag = "综艺"
        }
    }

    if typ != "" { q.Set("type", typ) }
    if tag != "" { q.Set("tag", tag) }
    if searchText != "" { q.Set("search_text", searchText) }
    q.Set("page_limit", pageLimit)
    q.Set("page_start", pageStart)

    target := "https://movie.douban.com/j/search_subjects?" + q.Encode()

    client := util.GetHTTPClient()
    req, err := http.NewRequest("GET", target, nil)
    if err != nil {
        c.JSON(http.StatusInternalServerError, model.NewErrorResponse(500, "创建请求失败: "+err.Error()))
        return
    }
    // 设置请求头，尽量模拟浏览器
    req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0 Safari/537.36")
    req.Header.Set("Accept", "application/json, text/plain, */*")
    req.Header.Set("Referer", "https://movie.douban.com/")

    resp, err := client.Do(req)
    if err != nil {
        c.JSON(http.StatusBadGateway, model.NewErrorResponse(502, "请求豆瓣失败: "+err.Error()))
        return
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        c.JSON(http.StatusBadGateway, model.NewErrorResponse(502, "读取豆瓣响应失败: "+err.Error()))
        return
    }

    if resp.StatusCode != 200 {
        c.JSON(http.StatusBadGateway, model.NewErrorResponse(502, "豆瓣响应状态异常: "+resp.Status))
        return
    }

    // 直接透传JSON
    c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// ImageProxyHandler 代理外部图片，主要用于豆瓣图片防盗链/跨域问题
func ImageProxyHandler(c *gin.Context) {
    raw := c.Query("u")
    if raw == "" {
        raw = c.Query("url")
    }
    if raw == "" {
        c.Status(http.StatusBadRequest)
        return
    }
    u, err := url.Parse(raw)
    if err != nil || !u.IsAbs() {
        c.Status(http.StatusBadRequest)
        return
    }
    host := strings.ToLower(u.Host)
    // 仅允许豆瓣相关域名，避免开放代理风险
    allowed := false
    if strings.HasSuffix(host, ".douban.com") || strings.HasSuffix(host, ".doubanio.com") {
        allowed = true
    }
    if !allowed {
        c.Status(http.StatusForbidden)
        return
    }

    client := util.GetHTTPClient()
    req, err := http.NewRequest("GET", u.String(), nil)
    if err != nil {
        c.Status(http.StatusInternalServerError)
        return
    }
    // 模拟浏览器，携带豆瓣Referer以通过防盗链
    req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0 Safari/537.36")
    req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")
    req.Header.Set("Referer", "https://movie.douban.com/")

    resp, err := client.Do(req)
    if err != nil {
        c.Status(http.StatusBadGateway)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        c.Status(http.StatusBadGateway)
        return
    }

    // 透传图片数据与类型
    ct := resp.Header.Get("Content-Type")
    if ct == "" {
        // 根据扩展名猜测类型
        ext := path.Ext(u.Path)
        if ext != "" {
            if t := mime.TypeByExtension(ext); t != "" { ct = t }
        }
        if ct == "" { ct = "image/jpeg" }
    }
    c.Header("Content-Type", ct)
    c.Header("Cache-Control", "public, max-age=86400")
    body, _ := io.ReadAll(resp.Body)
    c.Data(http.StatusOK, ct, body)
}
