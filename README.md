# 4caster

Telegram-бот, который предсказывает сколько будет длиться рендер
на основе параметров проекта.

Бот: [@hud0shnik_4caster_bot](https://t.me/hud0shnik_4caster_bot)

## Команды

- `/start` — описание и регистрация в боте
- Отправка параметров сцены/шейдера/времени кадра — бот вернёт оценку длительности

## Стек

- Go 1.26+
- [go-telegram-bot-api/v5](https://github.com/go-telegram-bot-api/telegram-bot-api)
- distroless/static контейнер
- Деплой: k3s

## Локальная разработка

```bash
cp .env.example .env
# впишите TELEGRAM_BOT_TOKEN от @BotFather

go run ./cmd
```

Или с переменной окружения напрямую:

```bash
TELEGRAM_BOT_TOKEN=... go run ./cmd
```

## Сборка Docker-образа

```bash
docker build -t hud0shnik/4caster:latest .
docker push hud0shnik/4caster:latest
```

Multi-stage: `golang:1.26.7-alpine` → `gcr.io/distroless/static:nonroot`,
статический бинарь, nonroot, ~20 MB.

## Деплой на k3s

Полная инструкция: [deploy/k3s/README.md](deploy/k3s/README.md)

Короткая версия (считаем, что k3s уже стоит):

```bash
git clone https://github.com/hud0shnik/4caster.git
cd 4caster

kubectl create namespace 4caster
kubectl -n 4caster create secret generic 4caster-telegram \
  --from-literal=TELEGRAM_BOT_TOKEN=<TOKEN_ОТ_BOTFATHER>

kubectl apply -k deploy/k3s
kubectl -n 4caster logs deploy/4caster --tail=20
```

После `git push` в `main` GitHub Actions собирает и пушит новый образ
в Docker Hub, watchdog в k3s раз в 5 минут делает `rollout restart`.

## CI/CD

`.github/workflows/docker.yml` собирает образ на каждый push в `main`
и тег `v*`, пушит в Docker Hub как `hud0shnik/4caster`. Нужны секреты:

- `DOCKERHUB_USERNAME` — логин Docker Hub
- `DOCKERHUB_TOKEN` — access token (https://hub.docker.com/settings/security)

## Структура проекта

```
.
├── cmd/main.go                 # точка входа бота
├── internal/handler/           # обработчики команд и сообщений
├── deploy/k3s/                 # манифесты для деплоя в k3s
│   ├── 00-namespace.yaml
│   ├── 05-rbac.yaml
│   ├── 10-secret.yaml
│   ├── 20-deployment.yaml
│   ├── 30-service.yaml
│   ├── 40-cronjob.yaml
│   ├── kustomization.yaml
│   └── README.md
├── Dockerfile                  # multi-stage, distroless
├── .github/workflows/docker.yml # сборка + push в Docker Hub
├── go.mod
└── LICENSE
```

## Лицензия

BSD 3-Clause. См. [LICENSE](LICENSE).