<template>
  <v-app>
    <AppBar />

    <v-main>
      <v-container>
        <v-row>
          <v-col cols="12" class="text-center">
            <h1 class="text-h3 font-weight-bold mb-2">Privacy Policy</h1>
            <p class="text-medium-emphasis mb-8">
              Last Updated: January 31, 2026
            </p>
          </v-col>
        </v-row>

        <v-row>
          <v-col cols="12" md="10" lg="8" class="mx-auto">
            <v-card class="pa-6">
              <div class="legal-content">
                <section class="mb-8">
                  <h2 class="text-h5 font-weight-bold mb-4">What Data We Collect</h2>
                  <p>
                    Anigraph is a simple anime list manager. Here's what we store:
                  </p>

                  <h3 class="text-h6 font-weight-bold mb-3 mt-6">If You Sign In With Google</h3>
                  <ul class="ml-6 mb-4">
                    <li>Your email address, name, and profile picture</li>
                    <li>We only ask for email and profile access - nothing else from your Google account</li>
                  </ul>

                  <h3 class="text-h6 font-weight-bold mb-3 mt-6">Your Lists & Preferences</h3>
                  <ul class="ml-6 mb-4">
                    <li>Anime you favorite</li>
                    <li>Custom lists you create (names, notes, privacy settings)</li>
                    <li>Your taste profile (automatically generated from your favorites)</li>
                    <li>Settings (like adult content filter)</li>
                  </ul>

                  <h3 class="text-h6 font-weight-bold mb-3 mt-6">Technical Stuff</h3>
                  <ul class="ml-6 mb-4">
                    <li>Session cookies (so you stay logged in for 30 days)</li>
                    <li>Security logs (to prevent abuse and spam)</li>
                    <li>Anonymous analytics via <a href="https://umami.is" target="_blank" rel="noopener" class="text-primary">Umami</a> (page views, feature usage - no personal info)</li>
                  </ul>
                </section>

                <section class="mb-8">
                  <h2 class="text-h5 font-weight-bold mb-4">How We Use Your Data</h2>
                  <ul class="ml-6 mb-4">
                    <li>To save your anime lists and favorites</li>
                    <li>To generate personalized recommendations</li>
                    <li>To keep you logged in</li>
                    <li>To prevent spam and abuse</li>
                  </ul>
                  <p class="mt-4">
                    <strong>We never:</strong>
                  </p>
                  <ul class="ml-6">
                    <li>Sell your data</li>
                    <li>Show you ads (at least not right now)</li>
                    <li>Share your info with third parties (except Google for login and Umami for anonymous analytics)</li>
                    <li>Track you across other websites</li>
                  </ul>
                </section>

                <section class="mb-8">
                  <h2 class="text-h5 font-weight-bold mb-4">Public Lists</h2>
                  <p>
                    If you make a list public or share it via link, anyone with the link can see the anime titles and notes in that list. Your email and personal info are never shown on public lists.
                  </p>
                </section>

                <section class="mb-8">
                  <h2 class="text-h5 font-weight-bold mb-4">Your Rights</h2>
                  <ul class="ml-6 mb-4">
                    <li>You can delete your lists anytime</li>
                    <li>You can use the site anonymously (without signing in)</li>
                    <li>To delete your account, email us at <strong>addisonbaum@gmail.com</strong></li>
                    <li>To export your data, email us and we'll send you a copy</li>
                  </ul>
                </section>

                <section class="mb-8">
                  <h2 class="text-h5 font-weight-bold mb-4">Security</h2>
                  <p>
                    We use HTTPS encryption, secure cookies, CSRF protection, and rate limiting to keep your data safe. But nothing on the internet is 100% secure.
                  </p>
                </section>

                <section class="mb-8">
                  <h2 class="text-h5 font-weight-bold mb-4">Changes</h2>
                  <p>
                    We may update this policy occasionally. We'll update the date at the top when we do.
                  </p>
                </section>

                <section class="mb-8">
                  <h2 class="text-h5 font-weight-bold mb-4">Questions?</h2>
                  <p>
                    Email us at <strong>addisonbaum@gmail.com</strong>
                  </p>
                </section>
              </div>
            </v-card>
          </v-col>
        </v-row>
      </v-container>
    </v-main>

    <AppFooter />
  </v-app>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'

onMounted(() => {
  document.title = 'Privacy Policy - Anigraph'
})
</script>

<style scoped>
.v-main {
  background-color: var(--color-bg);
  min-height: 100vh;
  padding-top: 64px;
  padding-bottom: 64px;
}

:deep(.v-card) {
  background: var(--gradient-surface-card);
  backdrop-filter: blur(10px);
  border: 1px solid var(--color-primary-border);
}

.legal-content {
  line-height: 1.8;
}

.legal-content h2 {
  color: var(--color-primary);
  margin-top: 2rem;
}

.legal-content h3 {
  color: var(--color-primary-hover);
}

.legal-content p {
  margin-bottom: 1rem;
  color: rgba(var(--color-text-rgb), 0.87);
}

.legal-content ul {
  margin-bottom: 1rem;
}

.legal-content li {
  margin-bottom: 0.5rem;
  color: rgba(var(--color-text-rgb), 0.87);
}

.legal-content a {
  text-decoration: underline;
}

.legal-content a:hover {
  opacity: 0.8;
}

.legal-content code {
  background: var(--color-primary-medium);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'Courier New', monospace;
  font-size: 0.9em;
}

.legal-content section:first-of-type {
  margin-top: 0;
}

.legal-content strong {
  color: rgba(var(--color-text-rgb), 0.95);
  font-weight: 600;
}
</style>
