FROM stratumn/gobase:0.2.0

MAINTAINER Stephan Florquin <stephan@stratumn.com>

RUN addgroup -S -g 999 stratumn
RUN adduser -H -D -u 999 -G stratumn stratumn

COPY LICENSE /usr/local/stratumn/
COPY dist/linux-amd64/{{CMD}} /usr/local/stratumn/bin/

RUN mkdir /usr/local/bin
RUN ln -s /usr/local/stratumn/bin/{{CMD}} /usr/local/bin/{{CMD}}

USER stratumn

CMD ["{{CMD}}"]
