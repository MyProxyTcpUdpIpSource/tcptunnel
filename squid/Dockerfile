FROM fedora

MAINTAINER Randomsock5 "Randomsock5@users.noreply.github.com"

RUN dnf install -y squid supervisor
RUN mkdir -p /var/log/supervisor
RUN mkdir -p /var/spool/squid

COPY supervisord.conf /etc/supervisor/conf.d/supervisor.conf
COPY squid.conf /etc/squid/squid.conf
COPY tt-server /opt/tcptunnel/tt-server
COPY run_squid.sh /opt/run_squid.sh
COPY tt-server.sh /opt/tt-server.sh

ENV SERVER_ADDRE 0.0.0.0
ENV SERVER_PORT  8443
ENV FORWARD  "127.0.0.1:3128"
ENV PASSWORD asdfghjkl

ENV SQUID_CONF /etc/squid/squid.conf

EXPOSE 3128
EXPOSE 8443

CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisor.conf"]
