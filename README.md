# Miniflux Sidekick

This is a sidekick container that runs alongside [Miniflux](https://miniflux.app) which is an Open Source RSS feed reader written in Go. You can check out the source code on [Github](https://github.com/miniflux/miniflux).

The goal is to support so called [Killfiles](https://en.wikipedia.org/wiki/Kill_file) to filter out items you don't want to see. You can think of it as an ad-blocker for your feed reader. The items are not deleted, they are just marked as *read* so you can still find all items in your feed reader if you have to.

## Supported Rules

The general format of the `killfile` is:

```
ignore-article "<feed>" "<filterexpr>"
```

### `<feed>`

This contains the URL of the feed that should be matched. It fuzzy matches the URL so if you only have one feed just use the base URL of the site. Example: `https://example.com` if the feed is on `https://example.com/rss/atom.xml`

### `<filterexpr>` Filter Expressions

From the [available rule set](https://newsboat.org/releases/2.15/docs/newsboat.html#_filter_language) and attributes (`Table 5. Available Attributes`) only a small subset are supported right now. These should cover most use cases already though.

**Attributes**

- `title`
- `content`

**Comparison Operators**

- `=~`: test whether regular expression matches
- `#`: contains; this operator matches if a word is contained in a list of space-separated words (useful for matching tags, see below)



## Example

Here's an example of what a `killfile` could look like with these rules. The first one marks all feed items as read that have a `[Sponsor]` string in the title. The second one filters out all feed items that have the word `Lunar` OR `moon` in there.


```
ignore-article "https://www.example.com" "title =~ \[Sponsor\]"
ignore-article "https://xkcd.com/atom.xml" "title # Lunar,Moon"
```
