# Opportunity Radar (Working Doc)

## What this is

This is a personal tool I'm building to make my job search and outreach easier.

I'll be the main (and initially only) user. The goal is to automate as much of the search and filtering work as possible, so I can spend my time actually applying and reaching out.

This document is intentionally lightweight and will evolve as the project evolves.

## The problem I'm solving

Right now:

- Job boards are noisy
- Searching for jobs to apply to is time-consuming
- Most roles shown to me are either unrealistic or low quality
- Finding small online-first companies to outreach to is manual and scattered
- There's no system that helps me decide what's worth spending more time on

The goal is to get not just more jobs to apply to, but better bets per unit of effort.

## What the tool should do (at a high level)

The app does two main things:

1. **Find jobs I have a reasonable chance of getting**
   - Kenya and remote
   - Junior / early-career friendly
   - Backend / generalist roles
   - Ranked so I can focus on the best ones

2. **Find small, online-first companies worth reaching out to**
   - SaaS, tools, platforms, online businesses
   - Not necessarily hiring
   - Likely to benefit from extra engineering help

The app is not meant to replace judgment — it's meant to reduce search and filtering work.

## How I'm approaching this

Instead of starting from companies I already know, the system starts from places where intent is visible. For example:

- Job boards and ATS feeds → hiring intent
- Product / startup directories → company activity
- Job posts used as signals, not just application targets

From there, everything gets:

- Normalized
- Filtered
- Scored using simple, explicit rules

## Rough architecture (will evolve)

At a conceptual level, the system looks like this:

- **Ingestors**  
  Fetch data from job boards, ATS feeds, and directories

- **Normalizers**  
  Convert messy external data into a small set of internal models

- **Scoring logic**  
  Rule-based heuristics to rank jobs and companies

- **Database**  
  Stores jobs, companies, and my decisions (applied, ignored, reached out, etc.)

- **Web UI**  
  Simple interface to review and act on opportunities

- **Scheduler**  
  Runs ingestion periodically without manual effort

This is all implemented as a single Go service for now.

## Tech choices (initial)

These are starting choices, not final commitments:

- **Backend:** Go
- **Database:** PostgreSQL
- **HTTP:** net/http with lightweight routing
- **Scraping:** standard HTTP + HTML parsing
- **UI:** server-rendered HTML
- **Deployment:** single service on a small cloud platform

The focus is reliability and clarity, not complexity.

## What this is not

- A public job board
- A LinkedIn scraper
- A large-scale SaaS
- A perfectly accurate recommendation system

## How I'll measure success

The project is successful if:

- It runs on its own once deployed
- It surfaces a manageable number of good opportunities weekly
- It meaningfully reduces the time I spend searching
- I actually use it during my job search

## Notes

This document will be updated as:

- Requirements change
- Sources are added or removed
- The architecture becomes clearer during implementation