# Security Policy

## Reporting a Vulnerability

The ObsFind team takes security vulnerabilities seriously. We appreciate your efforts to disclose your findings responsibly and will make every effort to acknowledge your contributions.

To report a security vulnerability, please follow these steps:

1. **Do not disclose the vulnerability publicly**
   - Please do not create a public GitHub issue for security vulnerabilities

2. **Email the security team**
   - Send your findings to: security@example.com
   - Use a descriptive subject line, e.g., "Security Vulnerability in ObsFind: XSS in Search Results"

3. **Include detailed information**
   - ObsFind version affected
   - Detailed steps to reproduce
   - Potential impact of the vulnerability
   - Any potential mitigations you've identified

## What to Expect

- **Initial Response**: We aim to acknowledge receipt of your vulnerability report within 48 hours.
- **Status Updates**: We will provide regular updates (at least once a week) about our progress addressing the issue.
- **Resolution Timeline**: We will work diligently to fix the vulnerability and release a patch as quickly as possible, typically within 90 days.

## Disclosure Policy

- We follow a coordinated disclosure process:
  1. We will acknowledge your report and confirm the vulnerability
  2. We will develop and test a fix
  3. We will release the fix and acknowledge your contribution (unless you prefer to remain anonymous)

## Security Updates

Security updates will be released as:

- Patch versions for the current stable release
- New minor versions for older releases if the vulnerability is severe
- Security advisories on GitHub

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

We generally provide security updates only for the most recent stable release and the prior stable release series.

## Security Best Practices

When deploying ObsFind in production, consider these security best practices:

1. Run the daemon with minimal permissions
2. Restrict network access to the daemon API
3. Keep all dependencies updated
4. Follow the security guidelines in the documentation

## Credits

We are grateful to the security researchers who have helped improve ObsFind's security. Researchers who report vulnerabilities will be acknowledged in our security advisories (unless they prefer to remain anonymous).