export ARCHS=linux/amd64 linux/arm darwin/amd64
export CGO_ENABLED=0

default:

jenkins:
	docker run --rm -i --name "make-deploy" \
		-e "GITHUB_TOKEN=$(GITHUB_TOKEN)" \
		-v "$(CURDIR):/go/src/github.com/contentflow/deploy" \
		-w /go/src/github.com/contentflow/deploy \
		--entrypoint /usr/bin/make \
		reporunner/golang-alpine \
		publish

publish:
	curl -sSLo golang.sh https://raw.githubusercontent.com/Luzifer/github-publish/master/golang.sh
	bash golang.sh
