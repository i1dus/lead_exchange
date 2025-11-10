# lead_exchange

Back-end сервиса "Биржа Лидов"

## Быстрый старт

```bash
git clone https://github.com/i1dus/lead_exchange.git

cp .env.example .env   
docker compose up --build
```

## Тестовые пользователи

Дефолтный пользователь: `user@m.c`:`password`

Админ: `admin@m.c`:`password`

## Swagger

Доступен по адресу: http://localhost:8081/swagger/index.html

Пока что всего 3 контракта (вбиваем в самый вверх):
1. `/swagger/auth/doc.json`
2. `/swagger/user/doc.json`
3. `/swagger/file/doc.json`

## Как работать с jwt токеном

Рекомендую установить расширение на браузер `ModHeader`

Дальше корректно вызываем ручку `auth/Login` и в ответе получаем тот самый токен. 

В ключ вставляем `Authorization`, а в значение `Bearer [полученный токен]`

[ ! ] Чтобы каждый раз не возиться с хедером, достаточно в качестве значения использовать `Bearer test`. Он будет равносилен тестовому пользователю.

## Как работать с S3

Для загрузки картинки используем ручку `file/UploadFile` или `file/UploadFiles`

В качестве аргументов:
- `fileName`: название файла
- `contentType`: тип файла (поддерживается `["jpeg", "png", "webp"]`)
- `file`: байтики в виде base64

В качестве ответа получаем ссылку на картинку, которую можно использовать на фронте в src 

