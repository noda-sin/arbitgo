FROM alpine:3.4

WORKDIR /app
COPY ./build/arbitgo /app

CMD ["./arbitgo"]