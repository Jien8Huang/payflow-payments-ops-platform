.PHONY: test-app fmt-app vet-app mod-verify-app build-dashboard

test-app:
	cd payflow-app && go test ./... -count=1

fmt-app:
	cd payflow-app && gofmt -s -w $$(find . -name '*.go' -not -path './vendor/*')

vet-app:
	cd payflow-app && go vet ./...

mod-verify-app:
	cd payflow-app && go mod verify

build-dashboard:
	cd payflow-dashboard && npm ci && npm run build
