# Context7 documentation policy

Use the Context7 MCP whenever the user asks about a library, framework, SDK, API, CLI tool, or cloud service. This includes questions about syntax, configuration, migrations, library-specific debugging, setup instructions, and CLI usage, even when the technology is familiar.

Prefer Context7 over web search for library documentation. Do not use it for refactoring, writing scripts from scratch, debugging business logic, code review, or general programming concepts.

## Workflow

1. Call `resolve-library-id` with the library name and the documentation topic, unless the user provides an exact library ID in `/org/project` format.
2. Choose the best match using, in order of relevance:
   - Exact library name
   - Description relevance
   - Code snippet count
   - Source reputation (prefer High or Medium)
   - Benchmark score (higher is better)
3. If the results are not a good match, try an alternate name or query (for example, `next.js` instead of `nextjs`) or rephrase the request. Use a version-specific ID when the user names a version.
4. Call `query-docs` with the selected library ID and a description of the documentation needed. Keep each query focused on one concept.
5. If the question covers multiple concepts, make a separate `query-docs` call for each concept using the same library ID. Combine concepts only when the user asks how they interact.
6. Answer using the documentation retrieved from Context7.
