# Maven Plugin for Relicta

Official Maven plugin for [Relicta](https://github.com/relicta-tech/relicta) - Publish artifacts to Maven Central (Java).

## Installation

```bash
relicta plugin install maven
relicta plugin enable maven
```

## Configuration

Add to your `release.config.yaml`:

```yaml
plugins:
  - name: maven
    enabled: true
    config:
      # Add configuration options here
```

## License

MIT License - see [LICENSE](LICENSE) for details.
