base_dir=$(shell pwd)

all: deps build zip

build: clean
	mkdir -p build
	GOOS=linux go build -o build/validate validate.go

clean:
	 rm -rf build

fmt:
	go fmt ${base_dir}

deps:
	go get github.com/aws/aws-sdk-go
	# go get github.com/aws/aws-lambda-go

zip:
	zip -j build/handler.zip build/validate

pipeline:
	echo "Setting up pipeline"
	cd ../; git clone git@github.com:tkashish/aws-ops.git
	cd ${base_dir}; ./pipeline_cf
