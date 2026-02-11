# Deployment Guide

## Creating a Release

This repository uses GitHub Actions to automatically build and release binaries when you create a new tag.

### Steps to Deploy

1. **Ensure your code is ready**
   ```bash
   # Make sure all changes are committed
   git status
   git add .
   git commit -m "Your commit message"
   ```

2. **Create a version tag**
   ```bash
   # Tag format: v{MAJOR}.{MINOR}.{PATCH}
   git tag v1.0.0
   ```

3. **Push the tag to GitHub**
   ```bash
   git push origin v1.0.0
   ```

4. **Wait for the build**
   - Go to the "Actions" tab in your GitHub repository
   - You'll see the "Build and Release" workflow running
   - Wait for it to complete (usually takes 2-3 minutes)

5. **Download the binary**
   - Go to the "Releases" section of your repository
   - Find your newly created release (e.g., v1.0.0)
    - Download the `ars-kit-v1.0.0-linux-amd64.tar.gz` file

### Version Naming Convention

Follow [Semantic Versioning](https://semver.org/):
- **MAJOR** version: Breaking changes (v2.0.0)
- **MINOR** version: New features, backward compatible (v1.1.0)
- **PATCH** version: Bug fixes (v1.0.1)

### Examples

```bash
# First release
git tag v1.0.0
git push origin v1.0.0

# Bug fix release
git tag v1.0.1
git push origin v1.0.1

# New feature release
git tag v1.1.0
git push origin v1.1.0

# Breaking changes
git tag v2.0.0
git push origin v2.0.0
```

### Installing the Binary

On the target Ubuntu 22.04 (Jammy) server:

```bash
# Download the release
wget https://github.com/ariesmaulana/ars-kit/releases/download/v1.0.0/ars-kit-v1.0.0-linux-amd64.tar.gz

# Verify checksum (optional but recommended)
wget https://github.com/ariesmaulana/ars-kit/releases/download/v1.0.0/ars-kit-v1.0.0-linux-amd64.tar.gz.sha256
sha256sum -c ars-kit-v1.0.0-linux-amd64.tar.gz.sha256

# Extract the binary
tar -xzf ars-kit-v1.0.0-linux-amd64.tar.gz

# Move to system path (optional)
sudo mv ars-kit /usr/local/bin/

# Make it executable (if needed)
chmod +x /usr/local/bin/ars-kit

# Verify installation
ars-kit --version  # or whatever command your app supports
```

### Troubleshooting

**Workflow fails to build:**
- Check the Actions tab for error logs
- Ensure `src/main.go` exists and compiles locally: `go build src/main.go`

**Release not created:**
- Ensure you have pushed the tag: `git push origin <tagname>`
- Check that the tag starts with `v`: `v1.0.0` not `1.0.0`

**Permission denied:**
- The workflow needs `contents: write` permission (already configured)
- Ensure GitHub Actions is enabled in repository settings

### Deleting a Release

```bash
# Delete the tag locally
git tag -d v1.0.0

# Delete the tag remotely
git push origin :refs/tags/v1.0.0

# Then manually delete the release from GitHub UI
```

## Workflow Details

The release workflow (`.github/workflows/release.yml`):
- Runs on Ubuntu 22.04 (Jammy)
- Uses Go 1.24
- Builds optimized binary with `-ldflags="-s -w"` (strips debug info)
- Creates compressed archive (.tar.gz)
- Generates SHA256 checksum
- Automatically creates GitHub release with downloadable assets
