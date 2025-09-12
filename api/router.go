package api

import (
	"pansou/config"
	"pansou/service"
	"pansou/util"

	"github.com/gin-gonic/gin"
)

// SetupRouter 设置路由
func SetupRouter(searchService *service.SearchService) *gin.Engine {
	// 设置搜索服务
	SetSearchService(searchService)
	
	// 设置为生产模式
	gin.SetMode(gin.ReleaseMode)
	
	// 创建默认路由
	r := gin.Default()
	
	// 添加中间件
	r.Use(CORSMiddleware())
	r.Use(LoggerMiddleware())
	r.Use(util.GzipMiddleware()) // 添加压缩中间件
	r.LoadHTMLGlob("templates/*.html")

    r.GET("/", DoubanPage)
    r.GET("/search", PansouPage)
    r.GET("/token", TokenPage)
    // 图片代理（不鉴权，解决豆瓣图片跨域/防盗链问题）
    r.GET("/img", ImageProxyHandler)
	
    // 定义无需鉴权的API
    r.GET("/api/douban", DoubanProxyHandler)
    r.POST("/api/token/verify", VerifyTokenHandler)

    // 定义需要鉴权的API路由组
    api := r.Group("/api")
    api.Use(AuthMiddleware())
    {
        // 搜索接口 - 支持POST和GET两种方式
        api.POST("/search", SearchHandler)
        api.GET("/search", SearchHandler) // 添加GET方式支持
        
        // 健康检查接口
        api.GET("/health", func(c *gin.Context) {
			// 根据配置决定是否返回插件信息
			pluginCount := 0
			pluginNames := []string{}
			pluginsEnabled := config.AppConfig.AsyncPluginEnabled
			
			if pluginsEnabled && searchService != nil && searchService.GetPluginManager() != nil {
				plugins := searchService.GetPluginManager().GetPlugins()
				pluginCount = len(plugins)
				for _, p := range plugins {
					pluginNames = append(pluginNames, p.Name())
				}
			}
			
			// 获取频道信息
			channels := config.AppConfig.DefaultChannels
			channelsCount := len(channels)
			
			response := gin.H{
				"status": "ok",
				"plugins_enabled": pluginsEnabled,
				"channels": channels,
				"channels_count": channelsCount,
			}
			
			// 只有当插件启用时才返回插件相关信息
			if pluginsEnabled {
				response["plugin_count"] = pluginCount
				response["plugins"] = pluginNames
			}
			
			c.JSON(200, response)
		})
	}
	
    return r
} 
