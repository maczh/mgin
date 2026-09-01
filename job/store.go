package job

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/logs"
	"gorm.io/gorm"
)

// 支持的存储驱动
const (
	DriverMysql    = "mysql"
	DriverPostgres = "postgres"
	DriverSqlite   = "sqlite"
)

// store 定时任务存储层，统一封装任务与执行日志的持久化操作
type store struct {
	db     *gorm.DB
	driver string
}

// newStore 按 MySQL → PostgreSQL → SQLite 的优先级选择当前可用的 GORM 数据库连接。
//
// 选择逻辑：
//  1. 依次检查 go.config.used 中是否启用了 mysql / postgres / sqlite；
//  2. 对启用的数据库尝试获取连接，成功即采用，失败则继续尝试下一个；
//  3. 三者都不可用时返回错误，定时任务模块不启动。
//
// 多库（multidb）模式下需要通过 go.job.dbName 指定使用哪个库；
// 未指定时自动取该数据库配置中的第一个连接。
func newStore(dbName string) (*store, error) {
	used := strings.ToLower(config.Config.Config.Used)
	var errs []string

	if strings.Contains(used, DriverMysql) {
		conn, err := mysqlConn(dbName)
		if err == nil && conn != nil {
			logs.Info("[Job] 定时任务存储使用 MySQL")
			return &store{db: conn, driver: DriverMysql}, nil
		}
		if err != nil {
			errs = append(errs, "MySQL: "+err.Error())
		}
	}
	if strings.Contains(used, DriverPostgres) {
		conn, err := postgresConn(dbName)
		if err == nil && conn != nil {
			logs.Info("[Job] 定时任务存储使用 PostgreSQL")
			return &store{db: conn, driver: DriverPostgres}, nil
		}
		if err != nil {
			errs = append(errs, "PostgreSQL: "+err.Error())
		}
	}
	if strings.Contains(used, DriverSqlite) {
		conn, err := db.Sqlite.GetConnection()
		if err == nil && conn != nil {
			logs.Info("[Job] 定时任务存储使用 SQLite")
			return &store{db: conn, driver: DriverSqlite}, nil
		}
		if err != nil {
			errs = append(errs, "SQLite: "+err.Error())
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("定时任务无可用数据库连接: %s", strings.Join(errs, "; "))
	}
	return nil, errors.New("定时任务需要在 go.config.used 中启用 mysql / postgres / sqlite 中的至少一种")
}

// mysqlConn 获取 MySQL 连接，兼容多库模式
func mysqlConn(dbName string) (*gorm.DB, error) {
	if db.Mysql.IsMultiDB() {
		if dbName == "" {
			names := db.Mysql.ListConnNames()
			if len(names) == 0 {
				return nil, errors.New("多库模式下无可用连接")
			}
			dbName = names[0]
		}
		return db.Mysql.GetConnection(dbName)
	}
	return db.Mysql.GetConnection()
}

// postgresConn 获取 PostgreSQL 连接，兼容多库模式
func postgresConn(dbName string) (*gorm.DB, error) {
	if db.Pg.IsMultiDB() {
		if dbName == "" {
			names := db.Pg.ListConnNames()
			if len(names) == 0 {
				return nil, errors.New("多库模式下无可用连接")
			}
			dbName = names[0]
		}
		return db.Pg.GetConnection(dbName)
	}
	return db.Pg.GetConnection()
}

// Driver 返回当前使用的数据库驱动名
func (s *store) Driver() string {
	return s.driver
}

// DB 返回底层 GORM 连接，便于业务侧做自定义查询
func (s *store) DB() *gorm.DB {
	return s.db
}

// autoMigrate 自动建表
func (s *store) autoMigrate() error {
	return s.db.AutoMigrate(&JobInfo{}, &JobLog{})
}

// ---------------------------------------------------------------- 任务

// listEnabled 查询所有启用中的任务
func (s *store) listEnabled() ([]JobInfo, error) {
	var jobs []JobInfo
	err := s.db.Where("status = ?", StatusRunning).Find(&jobs).Error
	return jobs, err
}

// listAll 查询所有任务（含已停止）
func (s *store) listAll() ([]JobInfo, error) {
	var jobs []JobInfo
	err := s.db.Find(&jobs).Error
	return jobs, err
}

// page 分页查询任务
func (s *store) page(group, keyword string, status, index, size int) ([]JobInfo, int64, error) {
	var (
		jobs  []JobInfo
		total int64
	)
	q := s.db.Model(&JobInfo{})
	if group != "" {
		q = q.Where("job_group = ?", group)
	}
	if status >= 0 {
		q = q.Where("status = ?", status)
	}
	if keyword != "" {
		kw := "%" + keyword + "%"
		q = q.Where("job_name LIKE ? OR handler_name LIKE ? OR description LIKE ?", kw, kw, kw)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if size <= 0 {
		size = 20
	}
	if index <= 0 {
		index = 1
	}
	err := q.Order("id DESC").Offset((index - 1) * size).Limit(size).Find(&jobs).Error
	return jobs, total, err
}

// getById 按 ID 查询任务
func (s *store) getById(id int64) (*JobInfo, error) {
	var j JobInfo
	err := s.db.Where("id = ?", id).First(&j).Error
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// getByName 按任务名查询任务
func (s *store) getByName(name string) (*JobInfo, error) {
	var j JobInfo
	err := s.db.Where("job_name = ?", name).First(&j).Error
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// create 新增任务
func (s *store) create(j *JobInfo) error {
	return s.db.Create(j).Error
}

// update 全量更新任务配置（不含统计字段与调度时间）
func (s *store) update(j *JobInfo) error {
	return s.db.Model(&JobInfo{}).Where("id = ?", j.ID).Updates(map[string]any{
		"job_group":        j.JobGroup,
		"description":      j.Description,
		"schedule_type":    j.ScheduleType,
		"schedule_conf":    j.ScheduleConf,
		"handler_name":     j.HandlerName,
		"job_param":        j.JobParam,
		"timeout":          j.Timeout,
		"retry_count":      j.RetryCount,
		"retry_interval":   j.RetryInterval,
		"block_strategy":   j.BlockStrategy,
		"misfire_strategy": j.MisfireStrategy,
		"remark":           j.Remark,
	}).Error
}

// remove 删除任务及其执行日志
func (s *store) remove(id int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", id).Delete(&JobInfo{}).Error; err != nil {
			return err
		}
		return tx.Where("job_id = ?", id).Delete(&JobLog{}).Error
	})
}

// updateStatus 更新任务启停状态
func (s *store) updateStatus(id int64, status int, next *time.Time) error {
	return s.db.Model(&JobInfo{}).Where("id = ?", id).Updates(map[string]any{
		"status":         status,
		"next_fire_time": next,
	}).Error
}

// updateFireTime 更新调度时间与累计调度次数
func (s *store) updateFireTime(id int64, last, next *time.Time) error {
	return s.db.Model(&JobInfo{}).Where("id = ?", id).
		Updates(map[string]any{
			"last_fire_time": last,
			"next_fire_time": next,
			"trigger_count":  gorm.Expr("trigger_count + 1"),
		}).Error
}

// updateNextTime 仅更新下次调度时间
func (s *store) updateNextTime(id int64, next *time.Time) error {
	return s.db.Model(&JobInfo{}).Where("id = ?", id).
		Update("next_fire_time", next).Error
}

// incrResult 累加成功/失败次数
func (s *store) incrResult(id int64, success bool) error {
	col := "fail_count"
	if success {
		col = "success_count"
	}
	return s.db.Model(&JobInfo{}).Where("id = ?", id).
		Update(col, gorm.Expr(col+" + 1")).Error
}

// ---------------------------------------------------------------- 执行日志

// createLog 新增执行日志，返回带自增 ID 的记录
func (s *store) createLog(l *JobLog) error {
	return s.db.Create(l).Error
}

// finishLog 回填执行结果
func (s *store) finishLog(l *JobLog) error {
	return s.db.Model(&JobLog{}).Where("id = ?", l.ID).Updates(map[string]any{
		"status":     l.Status,
		"end_at":     l.EndAt,
		"cost_ms":    l.CostMs,
		"message":    l.Message,
		"log_detail": l.LogDetail,
		"retry_num":  l.RetryNum,
	}).Error
}

// pageLog 分页查询执行日志
func (s *store) pageLog(jobId int64, jobName string, status, index, size int) ([]JobLog, int64, error) {
	var (
		logs  []JobLog
		total int64
	)
	q := s.db.Model(&JobLog{})
	if jobId > 0 {
		q = q.Where("job_id = ?", jobId)
	}
	if jobName != "" {
		q = q.Where("job_name = ?", jobName)
	}
	if status >= 0 {
		q = q.Where("status = ?", status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if size <= 0 {
		size = 20
	}
	if index <= 0 {
		index = 1
	}
	err := q.Order("id DESC").Offset((index - 1) * size).Limit(size).Find(&logs).Error
	return logs, total, err
}

// cleanLog 清理指定天数之前的执行日志，返回删除行数
func (s *store) cleanLog(days int) (int64, error) {
	if days <= 0 {
		return 0, nil
	}
	deadline := time.Now().AddDate(0, 0, -days)
	tx := s.db.Where("create_at < ?", deadline).Delete(&JobLog{})
	return tx.RowsAffected, tx.Error
}

// resetRunningLogs 将残留的「执行中」日志标记为失败。
// 用于进程非正常退出后重启时清理脏状态。
func (s *store) resetRunningLogs() (int64, error) {
	now := time.Now()
	tx := s.db.Model(&JobLog{}).Where("status = ?", LogRunning).Updates(map[string]any{
		"status":  LogFailed,
		"end_at":  &now,
		"message": "服务重启，执行状态丢失",
	})
	return tx.RowsAffected, tx.Error
}
