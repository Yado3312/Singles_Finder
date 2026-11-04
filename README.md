# Closer

Frontend Angular, API Go y MongoDB. El frontend consume `/api/profiles` mediante `ProfileApiService`. La imagen Docker compila Angular y Go y los sirve desde el mismo puerto.

## Inicio rápido con Docker

En macOS y Windows instala Docker Desktop y usa contenedores Linux. En Linux instala Docker Engine con el plugin Compose. Desde la raíz del repositorio, en los tres sistemas:

```sh
docker compose up --build -d
```

Abre `http://localhost:8080`. Compose levanta la aplicación y una instancia local de MongoDB, crea los índices y añade seis perfiles ficticios de forma repetible. **No necesitas credenciales de Atlas** para este modo.

Si el puerto `8080` ya está ocupado, define `APP_PORT=18080` antes de ejecutar Compose y abre `http://localhost:18080`. En PowerShell: `$env:APP_PORT='18080'`; en macOS/Linux: `export APP_PORT=18080`. Para revisar el estado y detener los servicios:

```sh
docker compose ps
docker compose down
```

Los datos locales quedan en el volumen `mongo_data` cuando detienes los contenedores.

## Usar MongoDB Atlas

Copia `backend/.env.example` a `backend/.env` si el archivo no existe. En PowerShell usa `Copy-Item backend/.env.example backend/.env`; en macOS/Linux usa `cp backend/.env.example backend/.env`. El archivo `.env` está excluido de Git y del contexto de Docker.

Reemplaza ambos campos entre `<>` en **una sola variable**:

```dotenv
MONGODB_URI=mongodb+srv://<db_username>:<db_password>@cluster0.gjyyl6k.mongodb.net/
```

Usa un usuario de **base de datos** de Atlas y codifica los caracteres especiales del usuario o contraseña para una URI. Atlas debe permitir la IP desde la que corre Docker. No copies credenciales al frontend, al Dockerfile ni a `.env.example`.

Para arrancar solo la aplicación conectada a Atlas:

```sh
docker compose -f compose.atlas.yaml up --build -d
```

Abre `http://localhost:8080`. Este modo crea los índices de las colecciones en Atlas, pero **no inserta perfiles ficticios por defecto**. Si deseas crearlos en Atlas, configura `ATLAS_SEED_DEMO=true` en tu entorno antes de ejecutar Compose. En PowerShell: `$env:ATLAS_SEED_DEMO='true'`. En macOS/Linux: `export ATLAS_SEED_DEMO=true`.

## Desarrollo sin Docker

Necesitas Node.js, npm y Go 1.25 o posterior. Configura `backend/.env` con Atlas o usa `DEMO_MODE=true` para perfiles ficticios en memoria.

Terminal 1:

```sh
cd backend
go run .
```

Terminal 2, desde la raíz:

```sh
npm ci
npm start
```

Abre `http://localhost:4200`. Angular envía `/api` a Go mediante `proxy.conf.json`.

## Alcance actual

- `GET /api/health`: comprueba la disponibilidad de Go y MongoDB.
- `POST /api/auth/register`, `POST /api/auth/login`, `POST /api/auth/logout` y `GET /api/auth/me`: registro, inicio/cierre de sesión y consulta de cuenta.
- `PUT /api/auth/quiz`: guarda intereses y preferencias del test inicial.
- `PUT /api/auth/settings`: actualiza el nombre de la cuenta.
- `GET /api/profiles?search=Sofia` y `GET /api/profiles/{id}`: consulta perfiles; con MongoDB requieren una sesión válida.
- [ProfileApiService](src/profile-api.service.ts): acceso reutilizable desde Angular.
- [Modelo de datos](docs/data-model.md): cuentas, perfiles, ubicaciones privadas, likes, matches, mensajes, bloqueos y reportes.

El registro valida que la persona sea mayor de 18 años. Las contraseñas se guardan con bcrypt; la sesión se entrega en una cookie `HttpOnly` y solo se conserva en MongoDB un hash de su token. Al entrar por primera vez, la persona debe completar el test de intereses y preferencias. La aplicación tiene únicamente las pestañas Mapa y Configuración.

El mapa pide permiso de ubicación al navegador y usa las coordenadas solo en el dispositivo para centrar la vista; no las envía ni las guarda en la cuenta. Si se deniega el permiso, el mapa sigue disponible en la vista inicial. La ubicación exacta nunca se comparte con otros perfiles.

`DEMO_MODE=true` conserva el modo de perfiles ficticios en memoria y **no habilita cuentas persistentes**. Para probar registro, test y sesión, usa MongoDB con `DEMO_MODE=false`. Likes persistentes, edición completa de perfil, mensajería, verificación de correo, recuperación de contraseña, moderación y límites de intentos siguen pendientes; la app aún no debe considerarse lista para producción con usuarios reales.

## Verificación

```sh
cd backend
go test ./...
CHECK_MONGODB=1 go test -run TestMongoConnection -count=1 -v
```

`TestMongoConnection` hace `ping` y lecturas, sin escribir datos. El build de Angular se verifica con `npm run build` desde la raíz.
