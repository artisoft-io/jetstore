build:
	GOOS=linux GOARCH=amd64 go build -o bootstrap main.go
	zip lambda-deployment-package.zip bootstrap

clean:
	rm -f bootstrap lambda-deployment-package.zip
