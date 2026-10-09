# AI-KBS Elasticsearch image with IK Analysis plugin pre-installed
#
# Pinned to a minor version (8.13.x) so the IK plugin build matches the ES
# distribution; bumping ES requires rebuilding this image. See
# require/04-architecture.md §3 (检索 + 持久化底座) and design.md §6.1
# (docker-compose.dev.yml — IK 分词插件预装).

FROM docker.elastic.co/elasticsearch/elasticsearch:8.13.4

# Install the IK analysis plugin from the official Elastic plugin mirror.
# The plugin version must match the ES distribution. Pin both to avoid
# silent upgrades on rebuild.
ARG ES_VERSION=8.13.4
ARG IK_VERSION=8.13.4

# elasticsearch-plugin refuses to run as root by default; the official image
# already runs as `elasticsearch` user, so we don't need a USER switch here.
# --batch avoids the interactive confirmation prompt.
RUN elasticsearch-plugin install --batch \
        https://github.com/medcl/elasticsearch-analysis-ik/releases/download/v${IK_VERSION}/elasticsearch-analysis-ik-${IK_VERSION}.zip

# Smoke check the plugin install (prints the installed plugin list to the
# build log so a silent mismatch fails the image build loudly).
RUN elasticsearch-plugin list | grep -i 'analysis-ik'