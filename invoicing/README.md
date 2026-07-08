# Country invoicing formats

Official e-invoice formats (Taiwan e-invoice, Japan qualified invoice, EU
PEPPOL, …), contributed **as data, not code**: the execution layer lives in
core and is validated against the golden vectors contributed here. If you know
the spec, you can contribute — no Go required.

One directory per format, named `<country>-<system>` (`tw-einvoice`,
`jp-qualified-invoice`, `eu-peppol`), containing:

- `README.md` — official spec link (version + date), coverage scope
- `format.json` / official XSD — target document structure
- `mapping.json` — MonetaKit canonical fields → format fields
- `vectors/` — golden pairs: canonical input JSON → expected output document
- `MAINTAINERS`

The mapping schema is still settling; open an issue before starting so we
shape it together. See [CONTRIBUTING.md](../CONTRIBUTING.md).
