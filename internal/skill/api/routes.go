package api

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册 Skill 路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	skillGroup := r.Group("/system/skills")
	skillGroup.Use(middleware.Auth())
	{
		skillGroup.GET("", ctrl.ListSkills)
		skillGroup.GET("/options", ctrl.GetSkillOptions)
		skillGroup.POST("", ctrl.CreateSkill)
		skillGroup.POST("/batch", ctrl.BatchCreateSkills)
		skillGroup.POST("/delete-batch", ctrl.DeleteSkillsBatch)
		skillGroup.POST("/import", ctrl.ImportSkill)
		skillGroup.POST("/import/prepare", ctrl.PrepareSkillUpload)

		skillGroup.GET("/dependency-options", ctrl.GetSkillDependencyOptions)
		skillGroup.GET("/builtin", ctrl.ListBuiltinSkills)
		skillGroup.POST("/builtin/sync", ctrl.SyncBuiltinSkills)
		skillGroup.GET("/:slug", ctrl.GetSkill)
		skillGroup.PUT("/:slug", ctrl.UpdateSkill)
		skillGroup.DELETE("/:slug", ctrl.DeleteSkill)
		skillGroup.GET("/:slug/export", ctrl.ExportSkill)
		skillGroup.GET("/:slug/tree", ctrl.GetSkillTree)
		skillGroup.GET("/:slug/file", ctrl.GetSkillFile)
		skillGroup.PUT("/:slug/file", ctrl.UpdateSkillFile)
		skillGroup.DELETE("/:slug/file", ctrl.DeleteSkillFile)
		skillGroup.PUT("/:slug/dependencies", ctrl.UpdateSkillDependencies)
		skillGroup.PUT("/:slug/enabled", ctrl.UpdateSkillEnabled)
	}

	// 用户级别路由
	userSkillGroup := r.Group("/skills")
	userSkillGroup.Use(middleware.Auth())
	{
		userSkillGroup.GET("/accessible", ctrl.ListAccessibleSkills)
		userSkillGroup.POST("/remote/list", ctrl.ListRemoteSkills)
		userSkillGroup.POST("/remote/prepare", ctrl.PrepareRemoteSkills)
	}
}
