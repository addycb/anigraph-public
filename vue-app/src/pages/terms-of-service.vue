<template>
  <v-app>
    <AppBar />

    <v-main>
      <v-container>
        <v-row>
          <v-col cols="12" class="text-center">
            <h1 class="text-h3 font-weight-bold mb-2">Terms of Service</h1>
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
                  <h2 class="text-h5 font-weight-bold mb-4">The Basics</h2>
                  <p>
                    By using Anigraph, you agree to these terms. Don't be a jerk, don't break the law, and don't try to hack or abuse the site. If you're under 13, you need parental permission to use this site.
                  </p>
                </section>

                <section class="mb-8">
                  <h2 class="text-h5 font-weight-bold mb-4">What Anigraph Does</h2>
                  <p>
                    Anigraph helps you discover anime and manage your lists. We don't host anime videos or copyrighted content - just metadata (titles, cover images, staff info) from public sources like AniList.
                  </p>
                </section>

                <section class="mb-8">
                  <h2 class="text-h5 font-weight-bold mb-4">Your Account</h2>
                  <ul class="ml-6 mb-4">
                    <li>You can use the site anonymously or sign in with Google</li>
                    <li>If you sign in with Google after using anonymously, your data will merge automatically</li>
                    <li>You're responsible for keeping your Google account secure</li>
                    <li>Don't create multiple accounts to bypass rate limits</li>
                  </ul>
                </section>

                <section class="mb-8">
                  <h2 class="text-h5 font-weight-bold mb-4">Don't Do These Things</h2>
                  <ul class="ml-6 mb-4">
                    <li>Don't scrape, bot, or spam the site</li>
                    <li>Don't try to hack or break the site</li>
                    <li>Don't create lists with offensive, illegal, or hateful content</li>
                    <li>Don't harass other users or steal their data</li>
                    <li>Don't use the site for commercial purposes without permission</li>
                  </ul>
                  <p class="mt-4">
                    If you violate these rules, we can delete your content or ban your account.
                  </p>
                </section>

                <section class="mb-8">
                  <h2 class="text-h5 font-weight-bold mb-4">Your Lists & Content</h2>
                  <ul class="ml-6 mb-4">
                    <li>You own your lists and notes</li>
                    <li>If you make a list public, anyone with the link can see it (but not your email or personal info)</li>
                    <li>By using Anigraph, you give us permission to store and display your content</li>
                    <li>We can remove content that violates these terms</li>
                  </ul>
                </section>

                <section class="mb-8">
                  <h2 class="text-h5 font-weight-bold mb-4">Disclaimers</h2>
                  <p>
                    <strong>The site is provided "as-is."</strong> We don't guarantee:
                  </p>
                  <ul class="ml-6 mb-4">
                    <li>That it will always be available or bug-free</li>
                    <li>That recommendations or data are 100% accurate</li>
                    <li>That your data is immune to loss (though we try our best)</li>
                  </ul>
                  <p class="mt-4">
                    <strong>Use at your own risk.</strong> We're not liable for data loss, service interruptions, or any damages. Since the site is free, the maximum we could owe you is $0.
                  </p>
                </section>

                <section class="mb-8">
                  <h2 class="text-h5 font-weight-bold mb-4">Termination</h2>
                  <ul class="ml-6 mb-4">
                    <li>You can stop using the site anytime</li>
                    <li>To delete your account, email <strong>addisonbaum@gmail.com</strong></li>
                    <li>We can ban accounts that violate these terms</li>
                  </ul>
                </section>

                <section class="mb-8">
                  <h2 class="text-h5 font-weight-bold mb-4">Changes</h2>
                  <p>
                    We can update these terms or change/shut down features at any time. We'll update the date at the top when we do. If you keep using the site after changes, you accept the new terms.
                  </p>
                </section>

                <section class="mb-8">
                  <h2 class="text-h5 font-weight-bold mb-4">Questions?</h2>
                  <p>
                    Email <strong>addisonbaum@gmail.com</strong>
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
  document.title = 'Terms of Service - Anigraph'
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
