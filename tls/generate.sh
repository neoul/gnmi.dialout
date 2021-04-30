#!/bin/sh

generate() {
  case "$1" in
  "ca-pki")
    openssl genrsa -out $CA_FILE.key 2048
    openssl req -new -x509 -days $CERT_DATE -key $CA_FILE.key -subj "$CA_C$CA_L$CA_O$CA_CN" -out $CA_FILE.crt
    ;;
  "server-pki")
    if [ -z "$2" ]; then
      IPADDR=$(ip addr | grep -Eo 'inet[6]{0,1} [0123456789abcdef:.]+' | awk '{print $2}')
      # echo $IPADDR
      COMMA=""
      for each in $IPADDR; do
        ADDR="${ADDR}${COMMA}IP:${each}"
        COMMA=","
      done
      ADDR="${ADDR}${COMMA}DNS:localhost"
      printf "subjectAltName=$ADDR" >san.txt
    else
      ADDR=""
      COMMA=""
      for each in $2; do
        echo $each
        ADDR="${ADDR}${COMMA}DNS:${each}"
        COMMA=","
      done
      printf "subjectAltName=$ADDR" >san.txt
    fi
    openssl req -newkey rsa:2048 -nodes -keyout $SERVER_CERT_FILE.key -subj "$SERVER_CERT_C$SERVER_CERT_L$SERVER_CERT_O$SERVER_CERT_CN" -out $SERVER_CERT_FILE.csr
    openssl x509 -req -extfile san.txt -days $CERT_DATE -in $SERVER_CERT_FILE.csr -CA $CA_FILE.crt -CAkey $CA_FILE.key -CAcreateserial -out $SERVER_CERT_FILE.crt
    rm -f san.txt
    ;;
  "client-pki")
    openssl req -newkey rsa:2048 -nodes -keyout $CLIENT_CERT_FILE.key -subj "$CLIENT_CERT_C$CLIENT_CERT_L$CLIENT_CERT_O$CLIENT_CERT_CN" -out $CLIENT_CERT_FILE.csr
    openssl x509 -req -days $CERT_DATE -in $CLIENT_CERT_FILE.csr -CA $CA_FILE.crt -CAkey $CA_FILE.key -CAcreateserial -out $CLIENT_CERT_FILE.crt -sha256 -
    ;;
  *)
    echo "unknown sub command input. It must be './generate.sh (ca-pki|server-pki|client-pki)'"
    exit 1
    ;;
  esac
}

CA_FILE=${CA_FILE:-"ca"}
# CA_C=${CA_C:-"/C=KR"}
# CA_L=${CA_L:-"/L=AY"}
CA_O=${CA_O:-"/O=HFR,Inc"}
CA_CN=${CA_CN:-"/CN=HFR NE(Network Equipment)'s self-signed CA"}

SERVER_CERT_FILE=${SERVER_CERT_FILE:-"server"}
# SERVER_CERT_C=${SERVER_CERT_C:-"/C=KR"}
# SERVER_CERT_L=${SERVER_CERT_L:-"/L=AY"}
SERVER_CERT_O=${SERVER_CERT_O:-"/O=HFR,Inc"}
# SERVER_CERT_CN=${SERVER_CERT_CN:-"HFR NE(Network Equipment) gNMI Target"}
SERVER_CERT_CN=${SERVER_CERT_CN:-"/CN=HFR-M6424"}

CLIENT_CERT_FILE=${CLIENT_CERT_FILE:-"client"}
# CLIENT_CERT_C=${CLIENT_CERT_C:-"/C=KR"}
# CLIENT_CERT_L=${CLIENT_CERT_L:-"/L=AY"}
CLIENT_CERT_O=${CLIENT_CERT_O:-"/O=HFR,Inc"}
CLIENT_CERT_CN=${CLIENT_CERT_CN:-"HFR NE(Network Equipment) gNMI Client"}

CERT_DATE=${CERT_DATE:-36500}
ADDR=${ADDR:-"DNS:localhost"}

if [ -z "$1" ]; then
  echo "no sub command input '$0 (ca-pki|server-pki (DOMAIN_NAME|)|client-pki)'"
  exit 1
fi

generate $1 $2
