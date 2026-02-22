# Skill: web-researcher

## Description
Deep web research combining search, fetching, and memory. Use when the user asks to research a topic, find information online, compare products/tools, investigate technical questions, or compile findings from multiple sources.

## Instructions
Combine `web_search`, `web_fetch`, and `memory_write` tools for thorough research workflows.

### Research Workflow

1. **Search broadly**: Use `web_search` with 2-3 varied queries to find relevant sources.
2. **Fetch key pages**: Use `web_fetch` on the most promising URLs to get full content.
3. **Synthesize**: Combine findings into a coherent summary.
4. **Save if valuable**: Use `memory_write` to persist important findings for future reference.

### Search Strategies

**Exploratory** (broad topic):
- Start with a general query, then refine based on results
- Try different phrasings: technical terms vs. plain language
- Search for "best practices", "comparison", "vs", "guide"

**Specific** (known target):
- Include exact names, versions, error messages
- Use site-specific searches when relevant
- Look for official docs first, then community resources

**Comparative** (evaluating options):
- Search for "[A] vs [B]" directly
- Look for benchmark data and real-world usage reports
- Check GitHub stars, last commit dates for project health

### Using web_search

Returns a list of URLs and snippets. Use to discover sources.
```
web_search: "kubernetes pod networking explained"
```

### Using web_fetch

Fetches and extracts readable content from a URL. Use on promising search results.
```
web_fetch: https://example.com/article
```

### Saving Research to Memory

When findings are worth keeping, write a structured summary to memory:
```
memory_write: research/topic-name.md
Content: key findings, links, dates, conclusions
```

### Research Report Format

When presenting findings, structure them as:
1. **Summary**: 2-3 sentence overview
2. **Key Findings**: Bulleted list of main points
3. **Sources**: Links to the most authoritative sources
4. **Caveats**: Limitations, conflicting information, or areas needing more research

### Tips
- Prefer recent sources (check dates)
- Cross-reference claims across multiple sources
- Note when information conflicts between sources
- Save research to memory when the user might need it again
- For technical topics, prefer official docs and reputable tech blogs over forums

## Tools
- name: research_topic
  description: Search for information on a topic
  command: web_search "{{query}}"
- name: fetch_page
  description: Fetch and read a web page
  command: web_fetch {{url}}
- name: save_research
  description: Save research findings to memory
  command: memory_write research/{{topic}}.md
