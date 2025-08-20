// Semantic Release Configuration for Alchemorsel v3
// Automated version management and release notes generation

module.exports = {
  branches: [
    'main',
    {
      name: 'develop',
      prerelease: 'beta'
    },
    {
      name: 'release/*',
      prerelease: 'rc'
    }
  ],
  
  plugins: [
    // Analyze commits to determine release type
    ['@semantic-release/commit-analyzer', {
      preset: 'conventionalcommits',
      releaseRules: [
        // Breaking changes
        { type: 'feat', scope: '*', breaking: true, release: 'major' },
        { type: 'fix', scope: '*', breaking: true, release: 'major' },
        { type: 'perf', scope: '*', breaking: true, release: 'major' },
        { type: 'refactor', scope: '*', breaking: true, release: 'major' },
        
        // New features
        { type: 'feat', release: 'minor' },
        
        // Bug fixes and improvements
        { type: 'fix', release: 'patch' },
        { type: 'perf', release: 'patch' },
        { type: 'security', release: 'patch' },
        
        // No release
        { type: 'docs', release: false },
        { type: 'style', release: false },
        { type: 'refactor', release: false },
        { type: 'test', release: false },
        { type: 'build', release: false },
        { type: 'ci', release: false },
        { type: 'chore', release: false },
        
        // Custom rules for Alchemorsel
        { scope: 'security', release: 'patch' },
        { scope: 'deps', release: 'patch' },
        { scope: 'api', breaking: true, release: 'major' },
        { scope: 'database', breaking: true, release: 'major' },
      ],
      parserOpts: {
        noteKeywords: ['BREAKING CHANGE', 'BREAKING CHANGES', 'BREAKING']
      }
    }],
    
    // Generate release notes
    ['@semantic-release/release-notes-generator', {
      preset: 'conventionalcommits',
      presetConfig: {
        types: [
          { type: 'feat', section: '✨ Features' },
          { type: 'fix', section: '🐛 Bug Fixes' },
          { type: 'perf', section: '⚡ Performance Improvements' },
          { type: 'security', section: '🔒 Security' },
          { type: 'refactor', section: '♻️ Code Refactoring' },
          { type: 'docs', section: '📚 Documentation' },
          { type: 'test', section: '🧪 Tests' },
          { type: 'build', section: '🏗️ Build System' },
          { type: 'ci', section: '🔄 Continuous Integration' },
          { type: 'chore', section: '🔧 Maintenance' },
          { type: 'style', section: '💄 Styles' },
          { type: 'revert', section: '⏪ Reverts' }
        ]
      },
      writerOpts: {
        commitPartial: `
## {{#if scope}}**{{scope}}:** {{/if}}{{subject}}

{{~!-- commit hash --}}
{{#if hash}}
\`{{hash}}\`
{{/if}}

{{~!-- commit references --}}
{{#if references~}}
  {{#references~}}
    {{#if @first~}}

*Closes*
    {{/if}}
    {{#if issue~}}
      {{issue}}
    {{else}}
      {{hash}}
    {{/if}}
    {{#unless @last}}, {{/unless}}
  {{/references}}
{{/if}}
        `
      }
    }],
    
    // Update CHANGELOG.md
    ['@semantic-release/changelog', {
      changelogFile: 'CHANGELOG.md',
      changelogTitle: '# Changelog\n\nAll notable changes to Alchemorsel v3 will be documented in this file.\n\nThe format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),\nand this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).'
    }],
    
    // Update package.json version
    ['@semantic-release/npm', {
      npmPublish: false,
      tarballDir: 'dist'
    }],
    
    // Commit changes back to repository
    ['@semantic-release/git', {
      assets: [
        'CHANGELOG.md',
        'package.json',
        'package-lock.json',
        'VERSION'
      ],
      message: 'chore(release): ${nextRelease.version} [skip ci]\n\n${nextRelease.notes}'
    }],
    
    // Create GitHub release
    ['@semantic-release/github', {
      assets: [
        {
          path: 'dist/*.tar.gz',
          label: 'Binary Archives'
        },
        {
          path: 'dist/*.zip', 
          label: 'Windows Archive'
        },
        {
          path: 'coverage.out',
          label: 'Test Coverage Report'
        }
      ],
      addReleases: 'bottom',
      draftRelease: false,
      discussionCategoryName: 'Releases',
      labels: ['release'],
      assignees: ['@alchemorsel/maintainers'],
      releasedLabels: ['released-v${nextRelease.version}'],
      successComment: `
🎉 This issue has been resolved in version \${nextRelease.version} 🎉

The release is available on:
- [GitHub Releases](https://github.com/alchemorsel/v3/releases/tag/v\${nextRelease.version})
- [Docker Hub](https://hub.docker.com/r/alchemorsel/v3/tags?name=\${nextRelease.version})

Your **[semantic-release](https://github.com/semantic-release/semantic-release)** bot 📦🚀
      `,
      failTitle: 'The automated release is failing 🚨',
      failComment: `
The automated release from the \`\${branch.name}\` branch failed. 🚨

I recommend you give this issue a high priority, so other packages depending on you could benefit from your bug fixes and new features.

You can find below the list of errors reported by **[semantic-release](https://github.com/semantic-release/semantic-release)**. Each one of them has to be resolved in order to automatically publish your package. I'm sure you can resolve this 💪.

Errors are usually caused by a misconfiguration or an authentication problem. With each error reported below you will find explanation and guidance to help you to resolve it.

Once all the errors are resolved, **[semantic-release](https://github.com/semantic-release/semantic-release)** will release your package the next time you push a commit to the \`\${branch.name}\` branch. You can also manually restart the failed CI job that runs **[semantic-release](https://github.com/semantic-release/semantic-release)**.

If you are not sure how to resolve this, here is some links that can help you:
- [Usage documentation](https://github.com/semantic-release/semantic-release#usage)
- [Frequently Asked Questions](https://github.com/semantic-release/semantic-release/blob/master/docs/support/FAQ.md)
- [Troubleshooting guide](https://github.com/semantic-release/semantic-release/blob/master/docs/support/troubleshooting.md)

If those don't help, or if this issue is reporting something you think isn't right, you can always ask the humans behind **[semantic-release](https://github.com/semantic-release/semantic-release)**.
      `
    }]
  ],
  
  // Configure when to trigger releases
  ci: true,
  debug: true,
  dryRun: false,
  
  // Environment variables configuration
  env: {
    GITHUB_TOKEN: process.env.GITHUB_TOKEN,
    GIT_AUTHOR_NAME: 'semantic-release-bot',
    GIT_AUTHOR_EMAIL: 'semantic-release-bot@alchemorsel.com',
    GIT_COMMITTER_NAME: 'semantic-release-bot',
    GIT_COMMITTER_EMAIL: 'semantic-release-bot@alchemorsel.com'
  }
};