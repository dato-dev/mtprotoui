package operation

const (
	TypeDeploy = "deploy"
	TypeDelete = "delete"

	DeployPreparing = "preparing"
	DeploySNI       = "sni"
	DeploySSH       = "ssh"
	DeployRun       = "run"
	DeployFinalize  = "finalize"
	DeployHealth    = "health"

	DeletePreparing = "preparing"
	DeleteSSH       = "ssh"
	DeleteContainer = "container"
	DeleteImage     = "image"
	DeleteFinalize  = "finalize"
)

type Step struct {
	Step     string
	Message  string
	Progress int
}

func DeploySteps() []Step {
	return []Step{
		{DeployPreparing, "Подготовка к развёртыванию...", 10},
		{DeploySNI, "Выбор SNI домена...", 20},
		{DeploySSH, "Подключение по SSH...", 30},
		{DeployRun, "Установка Docker и запуск прокси...", 55},
		{DeployFinalize, "Сохранение конфигурации...", 85},
		{DeployHealth, "Проверка доступности...", 95},
	}
}

func DeleteSteps() []Step {
	return []Step{
		{DeletePreparing, "Подготовка к удалению...", 10},
		{DeleteSSH, "Подключение по SSH...", 25},
		{DeleteContainer, "Остановка контейнера...", 50},
		{DeleteImage, "Удаление Docker-образа...", 75},
		{DeleteFinalize, "Удаление из консоли...", 90},
	}
}
