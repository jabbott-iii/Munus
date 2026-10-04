# Security Policy

Munus is a local, single-user command-line and terminal task manager. It runs no network
service, has no accounts or authentication, and stores tasks in a SQLite file on the user's
machine. This policy explains which versions receive security fixes and how to report a
vulnerability privately.

## Supported versions

Security fixes are made on `main` and shipped as a new patch release of the latest release
line. Older releases are not patched; upgrade to the latest release to get fixes.

| Version | Supported |
| ------- | --------- |
| 3.0.x   | Yes       |
| < 3.0   | No        |

## Reporting a vulnerability

**Do not report security vulnerabilities through public GitHub issues, pull requests or
discussions.**

Report them privately through GitHub's private vulnerability reporting:

1. Open the [Security tab](https://github.com/jabbott-iii/Munus/security) of this repository.
2. Select **Report a vulnerability**, or go straight to the
   [new advisory form](https://github.com/jabbott-iii/Munus/security/advisories/new).
3. Fill in the form. Only you and the maintainer can see the report.

If you cannot use GitHub's private reporting, open a public issue that asks for a private
contact channel, without any details of the vulnerability.

The rule in `CONTRIBUTING.md` that every change starts with a public issue does not apply to
security reports. If you are unsure whether something is a vulnerability, report it privately
anyway.

### What to include

- The affected version (`munus --version`), operating system and architecture, and whether you
  used a release binary, an image built from the `Dockerfile` or a build from source.
- The affected area, for example import, export, the TUI, terminal output, the database or
  backup files, the `Dockerfile` or the release workflow.
- Steps to reproduce, with a minimal input file or command line where relevant. Remove any real
  task data from samples.
- The impact: what an attacker can do, and what they need to control to do it.
- A suggested fix or mitigation, if you have one.

## What to expect

Munus is a small project with a single maintainer, so these are targets rather than
guarantees:

| Step | Target |
| ---- | ------ |
| Acknowledge the report | within 7 days |
| Initial assessment: confirmed, more information needed, or declined with reasons | within 14 days |
| Status updates while a fix is in progress | at least every 14 days |
| Fix released for a confirmed vulnerability | within 90 days of the report |

When a report is confirmed, the fix is developed privately in a GitHub security advisory and
released as a patch version. The advisory is then published with the affected and fixed
versions, and a CVE is requested through GitHub when the issue warrants one. You are credited
in the advisory unless you ask to remain anonymous.

When a report is declined, you get an explanation, for example that the behaviour is out of
scope or works as documented. You are welcome to follow up if you disagree.

Please keep the details private until the advisory is published. If no fix has been released
90 days after your report and you intend to disclose, let the maintainer know first so the
timing can be agreed.

## Scope

In scope are vulnerabilities in the code and release artifacts of this repository, including:

- parsing of import files and standard input (JSON decoding, size and task-count limits, text
  validation);
- task text written to the terminal (escape-sequence or control-character injection);
- permissions of the database, export and backup files and their directories, and the handling
  of database paths, including `MUNUS_DB_PATH`;
- deadline parsing or other input that can exhaust memory or crash Munus;
- the `Dockerfile` and the image it builds, for example privileges or data on its volume;
- the build and release pipeline: GitHub Actions workflows, release archives, `checksums.txt`
  and build provenance attestations;
- a known vulnerability in a dependency that is reachable from Munus code.

Out of scope:

- attacks that require already running code as the user, or controlling the user's home
  directory, data directory or environment, for example by pointing `MUNUS_DB_PATH` at a
  location other users can write;
- vulnerabilities in dependencies that Munus does not reach; report those to the upstream
  project;
- releases marked as unsupported above;
- output from automated scanners without a demonstrated impact on Munus.

## Good-faith research

If you make a good-faith effort to follow this policy, test only against your own
installation and data, and give the maintainer reasonable time to release a fix before
disclosure, the maintainer will not pursue or support legal action against you for that
research. Munus does not offer a bug bounty.

## Verifying releases

Release archives are published with `checksums.txt`, and releases from `v3.0.0` onward carry
a GitHub build provenance attestation. See [Install](README.md#install) for how to verify them.
