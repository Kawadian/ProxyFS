# Development secrets layout
#
# secrets/
#   dev/
#     master.key          # 32-byte key (gitignored) — used by compose hub service
#     master.key.example  # template for local setup
#
# Generate a dev master key:
#   openssl rand -hex 32 > secrets/dev/master.key
#   chmod 600 secrets/dev/master.key
