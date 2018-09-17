FROM alpine
# ENTRYPOINT ["/bin/registrator"]

COPY . /go/src/github.com/gliderlabs/registrator
RUN apk --no-cache add -t build-deps build-base go git \
	&& apk --no-cache add ca-certificates \
	&& cd /go/src/github.com/gliderlabs/registrator \
	&& export GOPATH=/go \
  && git config --global http.https://gopkg.in.followRedirects true \
	&& go build -ldflags "-X main.Version=$(cat VERSION)" -o /bin/registrator \
	&& rm -rf /go \
	&& apk del --purge build-deps

RUN apk add --no-cache jq curl unzip && \
	curl -fsSL https://goss.rocks/install | sh

ADD exmt /exmt

RUN chmod +x /exmt/entrypoint.sh

ENTRYPOINT [ "/exmt/entrypoint.sh" ]