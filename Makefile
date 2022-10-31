export ENV := develop
export DSN := username:password@tcp(localhost:3306)/widgets?parseTime=true&tls=false
export STRIPE_SECRET := sk_test_51LksyQJQyyUkN3mGazFaD2gdUk3BeriB0MCxp5zJ88by7jyhYmo6DFm438xfXeBdDMbz3Afww1IjovguyWHcqJau009QFSGxgX
export STRIPE_KEY := pk_test_51LksyQJQyyUkN3mGFxWqaWKm8qrOlgBWqeNgzChGgfRAFigvW5fPqKNhovBbrUQkywFmu0v0InjNzxgQe2CxODHm001BixUbJi
export SMTP_HOST := smtp.mailtrap.io
export SMTP_PORT := 25
export SMTP_USERNAME := 3839bb225b80a8
export SMTP_PASSWORD := e20e26115223d9
export SECRET_KEY := tv48oKVUjqXWRqasNBSMsbtAU7HaSiJk
export FRONTEND_PORT := 4000
export BACKEND_PORT := 4001
export INVOICE_PORT := 4002
export FRONTEND_URL := http://localhost
export BACKEND_URL := http://localhost

FRONTEND_BINARY=frontend
BACKEND_BINARY=backend
INVOICE_BINARY=invoice

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

## build the invoice microservice
build_invoice:
	@echo "Building invoice microservice..."
	@env CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/${INVOICE_BINARY} ./cmd/micro/invoice
	@echo "Invoice microservice built!"

## start the application
start: start_front start_back start_invoice

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

## start the invoice microservice
start_invoice: build_invoice
	@echo "Starting invoice microservice..."
	@env ./dist/${INVOICE_BINARY} -port=${INVOICE_PORT} &
	@echo "Invoice microservice started!"

## stop the application
stop: stop_front stop_back stop_invoice
	@echo "Application stopped"

## stop the front end
stop_front:
	@echo "Stopping front end..."
	@-pkill -SIGTERM -f "frontend"
	@echo "Front end stopped!"

## stop the back end
stop_back:
	@echo "Stopping back end..."
	@-pkill -SIGTERM -f "backend"
	@echo "Back end stopped!"

## stop the invoice microservice
stop_invoice:
	@echo "Stopping invoice microservice..."
	@-pkill -SIGTERM -f "invoice"
	@echo "Invoice microservice stopped!"

## stop the front end and then start
restart_front: stop_front start_front

## stop the back end and then start
restart_back: stop_back start_back

## stop the invoice microservice and then start
restart_invoice: stop_invoice start_invoice

## stop the application and then start
restart: stop start
