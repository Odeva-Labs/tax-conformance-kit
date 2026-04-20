# Tax Conformance Kit

TCK was built for Odeva Booking to test against the large ruleset of tourist tax that exists. There are thousands of edge cases (Between-country, overlapping tourist tax, special conditions, etc.) that we need to conform to. Instead of re-inventing the wheel, we use the dataset that is available to us and open-source libraries that others can use. We do it right once, so we don't need to do it again next year.

## Architecture

The project is composed of three layers:

- Spec (scraped from official publications)
- Engine (basic anomaly + input tests. You will throw your client against this)
- Client (the client that interacts with our tests. These are (preferably) embedded, rather than over-network.)

### Testing

A library will have a testing framework that is build in three layers:

#### 1. Unit tests

General interface testing that will test that _talking_ to the engine works.

#### 2. Conformance tests (cross-language)

These are fixture-driven legal scenarios, which are defined once and can be ran between other libraries. This way we do not need to write this logic in 5 different languages.

Part of this is a form of fuzzing where we check nice-to-have conditions (e.g. tourist tax will never be negative, it won't be >$1000 per guest, etc.).

#### 3. Corpus breadth

This is the big scary one. It defined the conditions like how many municipalities, years, clauses, exemptions, overlaps, tiers, seasonal windows are actually encoded. This is also where most of the work is done, and this will be hand-curated on every new update. Ironically, this is less work than having to automating it. Stuff goes wrong, the government isn't perfect. You know how it is.

## Countries supported

- [x] NL
- [x] ES (initial Catalonia + Balearic fixtures)
- [ ] DE
- [ ] BE
- [ ] LU
- [ ] missing country? add an issue with information!

For age-based regimes, conformance cases should include `booking_input.guests` with ages. Adult/child counts alone are not precise enough for rules like Catalonia's under-17 exemption or the Balearic under-16 exemption.

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
just runtime-resolve-evaluate
just runtime-evaluate-assessment
just runtime-resolve-evaluate-assessment
```

`runtime-resolve-evaluate` resolves the correct regulation fixture from `booking_input.property_location` and `stay_date`, then evaluates against it. This is the preferred entry point for multi-jurisdiction portfolios such as a Dutch operator with Spanish properties.

`runtime-resolve-evaluate-assessment` does the same resolution step for every booking in an assessment period, then returns one grouped filing result per resolved ruleset plus aggregate totals across the portfolio.

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
