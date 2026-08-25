# Deploy 4caster on k3s

## Prerequisites

- Сервер с установленным **k3s** (single-node или cluster).
- Токен бота от [@BotFather](https://t.me/BotFather).
- Docker Hub аккаунт, в котором уже опубликован образ `hud0shnik/myapp:latest`
  (пушится автоматически GitHub Actions на каждый push в `main`).

---

## 1. Доступ к кластеру с вашей машины

k3s кладёт kubeconfig в `/etc/rancher/k3s/k3s.yaml` на самом сервере. Скопируйте
его локально:

```bash
# На СЕРВЕРЕ (один раз)
sudo cat /etc/rancher/k3s/k3s.yaml
```

Скопируйте вывод, замените в нём `server: https://127.0.0.1:6443` на
`server: https://<IP_СЕРВЕРА>:6443` и сохраните локально как `~/.kube/config`:

```bash
mkdir -p ~/.kube
# отредактируйте файл и положите в ~/.kube/config
chmod 600 ~/.kube/config
```

Проверка:

```bash
kubectl get nodes
```

Должен быть один (или больше) `Ready` node.

---

## 2. Положить токен в Secret

```bash
kubectl create namespace 4caster

kubectl -n 4caster create secret generic 4caster-telegram \
  --from-literal=TELEGRAM_BOT_TOKEN=<TOKEN_ОТ_BOTFATHER>
```

Проверить:

```bash
kubectl -n 4caster get secret 4caster-telegram
```

---

## 3. Применить манифесты

```bash
kubectl apply -k deploy/k3s
```

Что создастся в namespace `4caster`:

| Ресурс | Назначение |
|---|---|
| `Deployment 4caster` | Под с ботом, образ `hud0shnik/myapp:latest` |
| `Service 4caster` | ClusterIP, для liveness/readiness проб |
| `Secret 4caster-telegram` | Токен бота |
| `ServiceAccount + Role + RoleBinding 4caster-rollout` | Права для watchdog |
| `CronJob 4caster-rollout-restart` | Раз в 5 минут проверяет новый image и делает `rollout restart` |

Проверка:

```bash
kubectl -n 4caster get all
kubectl -n 4caster logs deploy/4caster
```

В логах должна быть строка `authorized as <имя_бота>`.

---

## 4. Проверка watchdog

```bash
kubectl -n 4caster get cronjob
kubectl -n 4caster logs -l component=rollout-watchdog --tail=20
```

Ожидаемый лог (если образ не менялся):

```
image=hud0shnik/myapp:latest gen=N lastGen=N
up to date
```

---

## 5. Обновление бота

После каждого `git push` в `main`:

1. GitHub Actions собирает и пушит новый `hud0shnik/myapp:latest` в Docker Hub.
2. В течение 5 минут watchdog делает `kubectl rollout restart deploy/4caster`.
3. Под подхватывает свежий образ (`imagePullPolicy: Always`) и стартует.

Ручной форс-рестарт:

```bash
kubectl -n 4caster rollout restart deploy/4caster
```

---

## 6. Удаление

```bash
kubectl delete namespace 4caster
```

Удалит всё разом — деплоймент, service, secret, RBAC, cronjob.

---

## Troubleshooting

**Под в `ImagePullBackOff`** — образ приватный либо с другим именем.
Проверить:

```bash
kubectl -n 4caster describe pod -l app=4caster
```

Если у вас другой Docker Hub username — поправьте `kustomization.yaml`,
поле `newName`.

**Под `CrashLoopBackOff`** — бот не видит токен:

```bash
kubectl -n 4caster logs deploy/4caster
# "TELEGRAM_BOT_TOKEN is empty..."
```

Пересоздайте secret (см. шаг 2).

**Watchdog молчит** — прав ему хватает, но проверить:

```bash
kubectl -n 4caster logs -l component=rollout-watchdog
```

Если пусто — `kubectl -n 4caster get jobs` покажет, выполнялся ли он вообще.