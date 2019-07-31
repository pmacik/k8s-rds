FROM registry.access.redhat.com/ubi8/ubi
MAINTAINER Pavel Macík <pavel.macik@gmail.com>
ADD k8s-rds /k8s-rds
ENTRYPOINT ["/k8s-rds"]
