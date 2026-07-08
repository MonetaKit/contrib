# Tax filing & reporting formats

Country tax-report formats (VAT returns, sales-tax summaries, withholding
statements), same data-first contribution model as [`invoicing/`](../invoicing/README.md):
spec link, structure schema, field mapping, and golden vectors — the execution
layer lives in core.

One directory per format, named `<country>-<report>` (`tw-vat-401`,
`eu-vat-oss`), with the same file layout as invoicing formats.
