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
		skillGroup.DELETE("/batch", ctrl.BatchDeleteSkills)
		skillGroup.POST("/import", ctrl.ImportSkill)
		skillGroup.GET("/:slug", ctrl.GetSkill)
		skillGroup.PUT("/:slug", ctrl.UpdateSkill)
		skillGroup.DELETE("/:slug", ctrl.DeleteSkill)
		skillGroup.GET("/:slug/export", ctrl.ExportSkill)
		skillGroup.GET("/:slug/tree", ctrl.GetSkillTree)
		skillGroup.GET("/:slug/file", ctrl.GetSkillFile)
		skillGroup.PUT("/:slug/file", ctrl.UpdateSkillFile)
	}
}
