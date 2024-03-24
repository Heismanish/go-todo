# Base image
FROM golang:1.22-alpine

# Define work directory
WORKDIR /usr/src/app 

#  Copy
COPY . .

# Build and run
RUN go build -o main ./

EXPOSE 4000

ENV MONGO_URI=mongodb+srv://manishgu231:qRmSX2eMRmRE8eK4@cluster0.qyeqeis.mongodb.net/

CMD ["./main"]






