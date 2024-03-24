# Base image
FROM golang:1.22-alpine

# Define work directory
WORKDIR /usr/src/app 

#  Copy
COPY . .

# Build and run
RUN go build -o main ./

EXPOSE 4000

COPY .env /usr/src/app 


CMD ["./main"]






