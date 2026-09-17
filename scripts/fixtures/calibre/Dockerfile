# Disposable integration fixture, never a production deployment image.
FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends calibre ca-certificates \
    && rm -rf /var/lib/apt/lists/*
ENV QT_QPA_PLATFORM=offscreen
COPY docs/fixtures/e2e/librarry-public-domain-e2e-book.epub /fixture.epub
RUN calibre-server --version
COPY scripts/fixtures/calibre/start.sh /start.sh
CMD ["/bin/sh", "/start.sh"]
