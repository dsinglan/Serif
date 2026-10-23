# Full Stack Engineer Take-Home Assignment

_Copied from Serif Health's Ashby assignment page, as its Submission Guidelines ask._

## Searchable Price Transparency Explorer

Time budget: ~3 hours
Role: Full Stack Engineer
Skills evaluated: data ingestion, file parsing, backend/API design, frontend search and filtering, UI/UX, code structure, documentation, and pragmatic tradeoff decisions.

## Overview

For this exercise, you will build a small full-stack application that uses a Transparency in Coverage index file as a reference to find an in-network rate file link, ingests that file, exposes a simple API for searching, and displays formatted results in a UI.

The goal is to show how you think about working with CMS-style machine-readable files end to end:

1. Use the index file to identify a relevant in-network file URL
2. Load the referenced in-network rate file
3. Extract useful fields into a searchable shape
4. Expose an API endpoint for searching by code and optional provider identifiers
5. Present results in a formatted UI

This is intentionally open-ended. We care more about clear decisions, a working implementation, and a UI that makes the data easy to explore than about building a perfect production-grade pipeline.

## Data Source

Use this index file as the starting point for your solution:

```
https://www.centene.com/content/dam/centene/Centene%20Corporate/json/DOCUMENT/2026-08-31_fidelis_index.json
```

If link above is out of date then a new one can be found here: https://www.centene.com/price-transparency-files.html

This file follows the CMS Transparency in Coverage table-of-contents conventions described in the CMS guide:

https://github.com/CMSgov/price-transparency-guide

The index contains reporting plan metadata and references to one or more in-network files. Your solution should use the index as the source of truth for which rate files to load, but you do not need to handle every possible edge case in the CMS format.

## Requirements

Your submission should satisfy the following:

### Functional Requirements

1. Index File Reference
    - Use the index file above to identify relevant in-network file URL.
        - Can manually download and review index file to select an in-network file to use.
2. Rate File Ingestion
    - Fetch the referenced in-network rate file.
    - Normalize the data into a searchable shape.
3. API
    - Expose an API endpoint that returns search results from the normalized rate data.
        - The API should accept a required billing code and optional provider identifiers (for example NPI and/or EIN).
4. Searchable UI
    - Build a UI that allows rate searching.
        - Include a required search input for a specific billing code.
        - Support optional provider identifiers to narrow results (for example NPI and/or EIN).
    - Display search results in a formatted way.
5. Error Handling
    - Keep error handling basic so the app does not crash if a fetch or parse error is encountered.

## Deliverables

1. A git repo (GitHub, GitLab, etc.) containing your code
2. A README.md with:
    - Instructions to install and run
    - A short description of your architecture and data flow
    - Your assumptions, design decisions, and tradeoffs
    - What you would improve with more time
3. A working application that demonstrates the data ingestion and search experience

## Expectations

- Programming Language & Libraries: We use Vue.js and Golang internally, but you are welcome to submit in other languages, libraries, frameworks, or toolchains that you deem appropriate.
- Architecture: Choose a reasonable split between backend and frontend responsibilities. For example, you might use the backend to fetch and normalize the files and the frontend to query the resulting dataset.
- Data Storage: In-memory processing is acceptable for this take-home; you do not need to add a database. A database-backed approach can be treated as a follow-up extension for review discussion.
- Use of AI: You may use AI tools while building your submission. If you do, we will expect a more robust and polished search experience, including thoughtful handling of edge cases in user flows.
- Follow-Up Round: Candidates who move forward should expect a live code-pairing round where we add an additional feature to the submitted project.
- Time Investment: We are a small engineering team with limited resources, and often have to make hard tradeoffs to meet deadlines and make rapid forward progress. We do not want this takehome to take more than a few hours out of your day. Please timebox the technical work to 2–3 hours max, with additional time for documenting your learnings and decisions.

## Evaluation Criteria

| Area | Expectations |
|---|---|
| Functionality | Index is used to find an in-network file URL, rate file ingestion works, API search works, UI search works, optional provider identifier filtering works, and interactions are reliable |
| Code Quality | Clean, modular, readable, and appropriately structured |
| UI / UX | Clear presentation, useful search and filter controls, responsive layout |
| Data Handling | Sensible normalization, resilient parsing, good source traceability |
| Thoughtfulness | Good tradeoffs for CMS-style files, honest discussion of limitations |
| Documentation | Easy to run, clear README, concise explanation of the approach |

## Hints And Pointers

- The index is a table-of-contents style file and may reference multiple plans and file URLs.
- Negotiated rates are usually attached to in-network items/services and often nested under negotiated_rates and negotiated_prices style structures. You may need to walk multiple nested levels to connect code, provider group, and price.
- If provider identifiers are missing in one part of the file, check whether NPI or EIN appears in the related provider reference block instead of the negotiated price node itself.
- Consider whether you want to search by plan name, issuer, billing code, facility, location, or other fields exposed in the source files.
- If you make simplifying assumptions, call them out clearly in the README.

## Submission Guidelines

1. Repository Setup:
    - Please locally copy this takehome to your own public repo or import it to your GitHub account before sharing your solution.
    - Direct public forks and pull requests will expose your identity and solution to other candidates also working on this interview question, and we want the interview process to be fair for everyone.
2. README File:
    - Provide a README.md file at the root of your repository.
    - Include an overview of your approach, how to run the app, and any assumptions or tradeoffs that would help us evaluate your work.
3. Submission:
    - Once you have completed the assignment, share the link to your repository with us for evaluation through the Ashby submission link.

We look forward to reviewing your submission.
