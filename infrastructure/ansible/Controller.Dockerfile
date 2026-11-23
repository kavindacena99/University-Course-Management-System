# CLI/control node only: no Docker daemon or additional application service.
FROM docker:29-cli@sha256:34507eb5069d8be82c95dd3b6987d4250fd5fee49b82e2509b7d7a7030740d6d
RUN apk add --no-cache python3 py3-pip git \
    && python3 -m venv /opt/ansible
COPY infrastructure/scripts/requirements.txt /tmp/scripts/requirements.txt
COPY infrastructure/ansible/requirements.txt /tmp/ansible/requirements.txt
RUN /opt/ansible/bin/pip install --no-cache-dir -r /tmp/ansible/requirements.txt
ENV PATH="/opt/ansible/bin:${PATH}" ANSIBLE_LOCAL_TEMP=/tmp/ansible-local
WORKDIR /workspace
