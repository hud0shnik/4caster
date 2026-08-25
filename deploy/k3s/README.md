# Deploy 4caster on k3s

Целевой сценарий: пустой VPS с Ubuntu 22.04/24.04, нужен Telegram-бот,
который сам обновляется при `git push` в `main`.

---

## 0. Установка k3s на пустой VPS с Ubuntu

Подключаемся по SSH и ставим k3s одной командой (ставит и
сервер, и `kubectl`, и `crictl`, и Traefik как Ingress):

```bash
curl -sfL https://get.k3s.io | sh -
```

Проверить, что k3s поднялся:

```bash
sudo kubectl get nodes
# STATUS должен быть Ready

sudo kubectl get pods -A
# всё Running/Completed
```

Kubeconfig по умолчанию лежит в `/etc/rancher/k3s/k3s.yaml`. Чтобы
`kubectl` работал без `sudo`, копируем его себе:

```bash
mkdir -p ~/.kube
sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config
sudo chown $USER:$USER ~/.kube/config
chmod 600 ~/.kube/config
```

> На этом этапе `kubectl` крутится **на самом сервере**. Если хочется
> управлять с локальной машины — см. шаг 1.

---

## 1. (опционально) Доступ к кластеру с локальной машины

k3s кладёт kubeconfig в `/etc/rancher/k3s/k3s.yaml` на сервере. Скопируйте
его локально:

```bash
# На СЕРВЕРЕ
cat ~/.kube/config
```

Скопируйте вывод, замените в нём `server: https://127.0.0.1:6443` на
`server: https://<IP_СЕРВЕРА>:6443` и сохраните локально как
`~/.kube/config`:

```bash
mkdir -p ~/.kube
# сохраните отредактированный файл сюда
chmod 600 ~/.kube/config
kubectl get nodes
```

Должен быть один (или больше) `Ready` node.

Открыть API наружу на VDS (если управляете удалённо и порт 6443 закрыт):

```bash
sudo ufw allow 6443/tcp
sudo ufw reload
```

---

## 2. Склонировать репозиторий

```bash
git clone https://github.com/hud0shnik/4caster.git
cd 4caster
```

---

## 3. Положить токен в Secret

Получите токен у [@BotFather](https://t.me/BotFather) (`/newbot`,
`/token`).

```bash
kubectl create namespace 4caster

kubectl -n 4caster create secret generic 4caster-telegram \
  --from-literal=TELEGRAM_BOT_TOKEN=<TOKEN_ОТ_BOTFATHER>
```

Проверить:

```bash
kubectl -n 4caster get secret 4caster-telegram
```

> Если случайно создали с плейсхолдером — бот упадёт с `Not Found`.
> Пересоздать:
> ```bash
> kubectl -n 4caster delete secret 4caster-telegram
> kubectl -n 4caster create secret generic 4caster-telegram \
>   --from-literal=TELEGRAM_BOT_TOKEN=<настоящий_токен>
> ```

---

## 4. Применить манифесты

```bash
kubectl apply -k deploy/k3s
```

Что создастся в namespace `4caster`:

| Ресурс | Назначение |
|---|---|
| `Deployment 4caster` | Под с ботом, образ `hud0shnik/4caster:latest` |
| `Service 4caster` | ClusterIP, для liveness/readiness проб |
| `Secret 4caster-telegram` | Токен бота |
| `ServiceAccount + Role + RoleBinding 4caster-rollout` | Права для watchdog |
| `CronJob 4caster-rollout-restart` | Раз в 5 минут проверяет новый image и делает `rollout restart` |

Проверка:

```bash
kubectl -n 4caster get all
kubectl -n 4caster logs deploy/4caster --tail=20
```

В логах должно быть:

```
no .env file found, relying on system env
authorized as <имя_бота>
```

Если видите `Not Found` — токен неверный, см. шаг 3.

---

## 5. Проверка watchdog

```bash
kubectl -n 4caster get cronjob
kubectl -n 4caster logs -l component=rollout-watchdog --tail=20
```

Ожидаемый лог (если образ не менялся):

```
image=hud0shnik/4caster:latest gen=N lastGen=N
up to date
```

---

## 6. Обновление бота

После каждого `git push` в `main`:

1. GitHub Actions собирает и пушит новый `hud0shnik/4caster:latest`
   в Docker Hub.
2. В течение 5 минут watchdog делает `kubectl rollout restart deploy/4caster`.
3. Под подхватывает свежий образ (`imagePullPolicy: Always`) и стартует.

Не хотите ждать 5 минут — форс-рестарт руками:

```bash
kubectl -n 4caster rollout restart deploy/4caster
```

---

## 7. Удаление

```bash
kubectl delete namespace 4caster
```

Удалит всё разом — деплоймент, service, secret, RBAC, cronjob.

---

## Troubleshooting

**Под в `ErrImagePull` / `ImagePullBackOff`** — образ не стягивается.

```bash
kubectl -n 4caster describe pod -l app=4caster
```

Частые причины:

- Репозиторий приватный на Docker Hub. Сделайте `hud0shnik/4caster`
  публичным в https://hub.docker.com или добавьте `imagePullSecrets`.
- Неправильное имя образа. Текущее значение:
  ```bash
  kubectl -n 4caster get deploy 4caster \
    -o jsonpath='{.spec.template.spec.containers[0].image}'
  # должно быть: docker.io/hud0shnik/4caster:latest
  ```
  Если отличается — поправьте `newName` в `deploy/k3s/kustomization.yaml`
  и `kubectl apply -k deploy/k3s`.

**Бот стартует, но `Not Found` в логах** — токен неверный или секрет
создан с плейсхолдером. Пересоздайте secret (см. шаг 3).

**`CrashLoopBackOff` с `TELEGRAM_BOT_TOKEN is empty`** — секрета нет.
Пересоздайте (см. шаг 3).

**Watchdog молчит** — проверьте, запускался ли он вообще:

```bash
kubectl -n 4caster get jobs
kubectl -n 4caster logs -l component=rollout-watchdog
```

**Нет доступа к кластеру с локальной машины** — порт 6443 закрыт
фаерволом:

```bash
sudo ufw allow 6443/tcp
sudo ufw reload
```

И проверьте, что в локальном `~/.kube/config` `server:` указывает на
внешний IP сервера, а не `127.0.0.1`.