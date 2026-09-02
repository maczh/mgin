package job

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/errcode"
	"github.com/maczh/mgin/i18n"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models"
)

// RouterGroup 返回可挂载到任意 Gin 路由组下的定时任务管理接口。
//
// 提供的接口（以 group 的前缀为 /job 为例）：
//
//	GET    /job/list          任务列表（分页，支持 group/keyword/status 过滤）
//	GET    /job/:id           任务详情
//	POST   /job               新增任务
//	PUT    /job               更新任务
//	DELETE /job/:id           删除任务
//	POST   /job/:id/start     启动任务
//	POST   /job/:id/stop      停止任务
//	POST   /job/:id/trigger   手动触发一次
//	GET    /job/handlers      已注册执行器列表
//	GET    /job/log           执行日志列表（分页）
//
// 用法：
//
//	job.GetManager()            // 确保已初始化
//	r := router.Group("/job")
//	job.RouterGroup(r)
func RouterGroup(g *gin.RouterGroup) {
	g.GET("/list", listHandler)
	g.GET("/:id", detailHandler)
	g.POST("", createHandler)
	g.PUT("", updateHandler)
	g.DELETE("/:id", deleteHandler)
	g.POST("/:id/start", startHandler)
	g.POST("/:id/stop", stopHandler)
	g.POST("/:id/trigger", triggerHandler)
	g.GET("/handlers", handlersHandler)
	g.GET("/log", logHandler)
}

// parseID 从路径参数解析任务 ID
func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(200, i18n.Error(errcode.PARAM_ERROR, errcode.ParamError))
		return 0, false
	}
	return id, true
}

// parsePage 解析分页参数，缺省 index=1 size=20
func parsePage(c *gin.Context) (index, size int) {
	index, _ = strconv.Atoi(c.DefaultQuery("index", "1"))
	size, _ = strconv.Atoi(c.DefaultQuery("size", "20"))
	if index <= 0 {
		index = 1
	}
	if size <= 0 {
		size = 20
	}
	return
}

// listHandler 任务列表（分页）
func listHandler(c *gin.Context) {
	m := GetManager()
	st := m.Store()
	if st == nil {
		c.JSON(200, i18n.Error(errcode.STORAGE_ERROR, errcode.StorageError))
		return
	}
	group := c.Query("group")
	keyword := c.Query("keyword")
	status, _ := strconv.Atoi(c.Query("status"))
	index, size := parsePage(c)
	list, total, err := st.page(group, keyword, status, index, size)
	if err != nil {
		c.JSON(200, i18n.Error(errcode.JOB_ERROR, err.Error()))
		return
	}
	page := &models.ResultPage{Count: int(total + int64(size) - 1) / size, Index: index, Size: size, Total: int(total)}
	c.JSON(200, models.SuccessWithPage(list, page.Count, page.Index, page.Size, page.Total))
}

// detailHandler 任务详情
func detailHandler(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	st := GetManager().Store()
	if st == nil {
		c.JSON(200, i18n.Error(errcode.STORAGE_ERROR, errcode.StorageError))
		return
	}
	job, err := st.getById(id)
	if err != nil {
		c.JSON(200, i18n.Error(errcode.JOB_ERROR, "任务不存在"))
		return
	}
	c.JSON(200, models.Success(job))
}

// createHandler 新增任务
func createHandler(c *gin.Context) {
	var j JobInfo
	if err := c.ShouldBindJSON(&j); err != nil {
		c.JSON(200, i18n.Error(errcode.PARAM_ERROR, err.Error()))
		return
	}
	if j.JobName == "" || j.ScheduleConf == "" || j.HandlerName == "" {
		c.JSON(200, i18n.Error(errcode.PARAM_ERROR, errcode.ParamError))
		return
	}
	if j.ScheduleType == "" {
		j.ScheduleType = ScheduleCron
	}
	if j.BlockStrategy == "" {
		j.BlockStrategy = BlockSerial
	}
	if j.MisfireStrategy == "" {
		j.MisfireStrategy = MisfireDoNothing
	}
	st := GetManager().Store()
	if st == nil {
		c.JSON(200, i18n.Error(errcode.STORAGE_ERROR, errcode.StorageError))
		return
	}
	if err := st.create(&j); err != nil {
		c.JSON(200, i18n.Error(errcode.JOB_ERROR, err.Error()))
		return
	}
	// 同步到内存调度器（不重启即生效）
	if m := GetManager(); m.IsRunning() {
		if err := m.reload(); err != nil {
			logs.Warn("[Job] 新增任务后同步失败: {}", err.Error())
		}
	}
	c.JSON(200, models.Success(j))
}

// updateHandler 更新任务
func updateHandler(c *gin.Context) {
	var j JobInfo
	if err := c.ShouldBindJSON(&j); err != nil {
		c.JSON(200, i18n.Error(errcode.PARAM_ERROR, err.Error()))
		return
	}
	if j.ID <= 0 {
		c.JSON(200, i18n.Error(errcode.PARAM_ERROR, errcode.ParamError))
		return
	}
	st := GetManager().Store()
	if st == nil {
		c.JSON(200, i18n.Error(errcode.STORAGE_ERROR, errcode.StorageError))
		return
	}
	old, err := st.getById(j.ID)
	if err != nil {
		c.JSON(200, i18n.Error(errcode.JOB_ERROR, "任务不存在"))
		return
	}
	// 保留不允许通过接口修改的字段
	j.Status = old.Status
	j.LastFireTime = old.LastFireTime
	j.NextFireTime = old.NextFireTime
	j.TriggerCount = old.TriggerCount
	j.SuccessCount = old.SuccessCount
	j.FailCount = old.FailCount
	if err = st.update(&j); err != nil {
		c.JSON(200, i18n.Error(errcode.JOB_ERROR, err.Error()))
		return
	}
	if m := GetManager(); m.IsRunning() {
		_ = m.reload()
	}
	c.JSON(200, models.Success(j))
}

// deleteHandler 删除任务
func deleteHandler(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	st := GetManager().Store()
	if st == nil {
		c.JSON(200, i18n.Error(errcode.STORAGE_ERROR, errcode.StorageError))
		return
	}
	if err := st.remove(id); err != nil {
		c.JSON(200, i18n.Error(errcode.JOB_ERROR, err.Error()))
		return
	}
	if m := GetManager(); m.IsRunning() {
		_ = m.reload()
	}
	c.JSON(200, models.Success[any](nil))
}

// startHandler 启动任务
func startHandler(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	st := GetManager().Store()
	if st == nil {
		c.JSON(200, i18n.Error(errcode.STORAGE_ERROR, errcode.StorageError))
		return
	}
	if err := st.updateStatus(id, StatusRunning, nil); err != nil {
		c.JSON(200, i18n.Error(errcode.JOB_ERROR, err.Error()))
		return
	}
	if m := GetManager(); m.IsRunning() {
		_ = m.reload()
	}
	c.JSON(200, models.Success[any](nil))
}

// stopHandler 停止任务
func stopHandler(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	st := GetManager().Store()
	if st == nil {
		c.JSON(200, i18n.Error(errcode.STORAGE_ERROR, errcode.StorageError))
		return
	}
	if err := st.updateStatus(id, StatusStopped, nil); err != nil {
		c.JSON(200, i18n.Error(errcode.JOB_ERROR, err.Error()))
		return
	}
	if m := GetManager(); m.IsRunning() {
		_ = m.reload()
	}
	c.JSON(200, models.Success[any](nil))
}

// triggerHandler 手动触发一次
func triggerHandler(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body struct {
		Param map[string]interface{} `json:"param"`
	}
	_ = c.ShouldBindJSON(&body)
	logID, err := GetManager().Trigger(id, body.Param)
	if err != nil {
		c.JSON(200, i18n.Error(errcode.JOB_ERROR, err.Error()))
		return
	}
	c.JSON(200, models.SuccessWithMsg("已触发", gin.H{"logId": logID}))
}

// handlersHandler 已注册执行器列表
func handlersHandler(c *gin.Context) {
	c.JSON(200, models.Success(ListHandlers()))
}

// logHandler 执行日志列表（分页）
func logHandler(c *gin.Context) {
	st := GetManager().Store()
	if st == nil {
		c.JSON(200, i18n.Error(errcode.STORAGE_ERROR, errcode.StorageError))
		return
	}
	jobId, _ := strconv.ParseInt(c.Query("jobId"), 10, 64)
	jobName := c.Query("jobName")
	status, _ := strconv.Atoi(c.Query("status"))
	index, size := parsePage(c)
	list, total, err := st.pageLog(jobId, jobName, status, index, size)
	if err != nil {
		c.JSON(200, i18n.Error(errcode.JOB_ERROR, err.Error()))
		return
	}
	page := &models.ResultPage{Count: int(total + int64(size) - 1) / size, Index: index, Size: size, Total: int(total)}
	c.JSON(200, models.SuccessWithPage(list, page.Count, page.Index, page.Size, page.Total))
}
