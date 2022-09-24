export DSN := username:password@tcp(localhost:3306)/widgets?parseTime=true&tls=false
export STRIPE_SECRET := sk_test_51LksyQJQyyUkN3mGazFaD2gdUk3BeriB0MCxp5zJ88by7jyhYmo6DFm438xfXeBdDMbz3Afww1IjovguyWHcqJau009QFSGxgX
export STRIPE_KEY := pk_test_51LksyQJQyyUkN3mGFxWqaWKm8qrOlgBWqeNgzChGgfRAFigvW5fPqKNhovBbrUQkywFmu0v0InjNzxgQe2CxODHm001BixUbJi
FRONTEND_BINARY=frontend
BACKEND_BINARY=backend
FRONTEND_PORT=4000
BACKEND_PORT=4001

## clean all binaries and run go clean
clean:
	@echo "Cleaning..."
	@rm -rf dist
	@go clean
	@echo "Cleaned!"

## build the front end
build_front:
	@echo "Building front end..."
	@env CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/${FRONTEND_BINARY} ./cmd/web
	@echo "Front end built!"

## build the back end
build_back:
	@echo "Building back end..."
	@env CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/${BACKEND_BINARY} ./cmd/api
	@echo "Back end built!"

## start the application
start: start_front start_back

## start the front end
start_front: build_front
	@echo "Starting front end..."
	@env STRIPE_KEY=${STRIPE_KEY} STRIPE_SECRET=${STRIPE_SECRET} ./dist/${FRONTEND_BINARY} -port=${FRONTEND_PORT} &
	@echo "Front end started!"

## start the back end
start_back: build_back
	@echo "Starting back end..."
	@env STRIPE_KEY=${STRIPE_KEY} STRIPE_SECRET=${STRIPE_SECRET} ./dist/${BACKEND_BINARY} -port=${BACKEND_PORT} &
	@echo "Back end started!"

## stop the application
stop: stop_front stop_back
	@echo "Application stopped"

## stop the front end
stop_front:
	@echo "Stopping front end..."
	@-pkill -SIGTERM -f "frontend -port=${FRONTEND_PORT}"
	@echo "Front end stopped!"

## stop the back end
stop_back:
	@echo "Stopping back end..."
	@-pkill -SIGTERM -f "backend -port=${BACKEND_PORT}"
	@echo "Back end stopped!"

## stop the front end and then start
restart_front: stop_front start_front

## stop the back end and then start
restart_back: stop_back start_back

## stop the application and then start
restart: stop start