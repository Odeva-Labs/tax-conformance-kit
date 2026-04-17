# Tax Conformance Kit

TCK was built for Odeva Booking to test against the large ruleset of tourist tax that exists. There are thousands of edge cases (Between-country, overlapping tourist tax, special conditions, etc.) that we need to conform to. Instead of re-inventing the wheel, we use the dataset that is available to us and open-source libraries that others can use. We do it right once, so we don't need to do it again next year.

## Architecture

A library will have a testing framework that is build in three layers:

### 1. Unit tests

General interface testing that will test that *talking* to the engine works.

### 2. Conformance tests (cross-language)

These are fixture-driven legal scanrios, which is defined once and can be ran between other libraries. This way we do not need to write 5 different libraries.

Part of this is a form of fuzzing where we check nice-to-have conditions (e.g. tourist tax will never be negative).

### 3. Corpus breadth

This is the big scary one. It defined the conditions like how many municipalities, years, clauses, exemptions, overlaps, tiers, seasonal windows are actually encoded. This is also where most of the work is done, and this will be hand-curated on every new update. Ironically, this is less work than having to automating it. Stuff goes wrong, the government isn't perfect. You know how it is.

## Start

Requires `go 1.23`.

```bash
just test
just test-ruby-client
just test-ruby-gem
just validate
just evaluate-assessment
```

## App Integration

```bash
just runtime-validate
just runtime-evaluate
just runtime-evaluate-assessment
```

## Draft Fixture Pipeline

```bash
just harvest
just select
just extract
just analyze
just generate
```

If the CBS municipality dataset changes, refresh the code catalog with:

```bash
just import-municipality-codes /path/to/86247NED.zip
just backfill-municipality-codes
```

## Sources

- CVDR SRU search API: `https://zoekdienst.overheid.nl/sru/Search?x-connection=cvdr`
- Official regulation publications: `https://lokaleregelgeving.overheid.nl/CVDR...`
- CBS municipality dataset `86247NED`: `https://datasets.cbs.nl/CSV/CBS/nl/86247NED`
