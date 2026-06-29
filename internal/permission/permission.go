package permission

const (
	SystemUserRead         = "system.user.read"
	SystemUserManage       = "system.user.manage"
	SystemRoleManage       = "system.role.manage"
	SystemTemplateManage   = "system.template.manage"
	SystemSharedPoolManage = "system.shared_pool.manage"

	AppCreate       = "app.create"
	AppRead         = "app.read"
	AppUpdate       = "app.update"
	AppDelete       = "app.delete"
	AppMemberRead   = "app.member.read"
	AppMemberManage = "app.member.manage"

	EnvCreate = "env.create"
	EnvRead   = "env.read"
	EnvManage = "env.manage"
	EnvDelete = "env.delete"

	ServiceRead    = "service.read"
	ServiceInstall = "service.install"
	ServiceManage  = "service.manage"

	ComponentRead   = "component.read"
	ComponentCreate = "component.create"
	ComponentDeploy = "component.deploy"
	ComponentManage = "component.manage"
)
