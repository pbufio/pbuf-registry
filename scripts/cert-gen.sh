#!/bin/sh

CERT_GEN_PATH=./gen/certs

rm -f $CERT_GEN_PATH

mkdir -p $CERT_GEN_PATH

echo "subjectAltName=DNS:pbuf.cloud" > $CERT_GEN_PATH/extfile.cnf

openssl req -x509 -newkey rsa:4096 -days 365 -nodes -keyout $CERT_GEN_PATH/ca-key.pem -out $CERT_GEN_PATH/ca-cert.pem -subj "/CN=pbuf.cloud/emailAddress=hello@pbuf.io"
openssl x509 -in $CERT_GEN_PATH/ca-cert.pem -noout -text

openssl req -newkey rsa:4096 -nodes -keyout $CERT_GEN_PATH/server-key.pem -out $CERT_GEN_PATH/server-req.pem -subj "/CN=pbuf.cloud/emailAddress=hello@pbuf.io"
openssl x509 -req -in $CERT_GEN_PATH/server-req.pem -days 60 -CA $CERT_GEN_PATH/ca-cert.pem -CAkey $CERT_GEN_PATH/ca-key.pem -CAcreateserial -out $CERT_GEN_PATH/server-cert.pem -extfile $CERT_GEN_PATH/extfile.cnf

openssl x509 -in $CERT_GEN_PATH/server-cert.pem -noout -text